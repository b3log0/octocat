package main

import (
	"encoding/base64"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
)

const (
	UserAgent = "Octocat/1.0.0; +https://github.com/b3log/octocat"
)

var logger *Logger

func init() {
	rand.Seed(time.Now().Unix())

	SetLevel("info")
	logger = NewLogger(os.Stdout)
	gin.SetMode(gin.ReleaseMode)
}

func mapRoutes() *gin.Engine {
	ret := gin.New()
	ret.Use(gin.Recovery())

	ret.POST("/github/repos/solo", pushRepos)
	ret.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "The piper will lead us to reason.\n\n欢迎访问黑客与画家的社区 https://hacpai.com")
	})

	return ret
}

func pushRepos(c *gin.Context) {
	result := NewResult()
	result.Code = CodeErr

	ak := c.PostForm("ak")
	user := user(ak)
	if nil == user {
		result.Msg = "get user failed"
		c.JSON(http.StatusOK, result)

		return
	}
	user["ak"] = ak

	file, err := c.FormFile("file")
	if nil != err {
		result.Msg = "get file failed"
		c.JSON(http.StatusOK, result)

		return
	}
	if 128 > file.Size {
		result.Msg = "file is too small"
		c.JSON(http.StatusOK, result)

		return
	}

	name := c.PostForm("repoName")
	readme := c.PostForm("repoReadme")
	description := c.PostForm("repoDesc")
	homepage := c.PostForm("repoHomepage")

	repo := createOrUpdateRepo(user, name, description, homepage)
	if nil == repo {
		result.Msg = "create or update repo failed"
		c.JSON(http.StatusOK, result)

		return
	}

	updateREADME(user, name, readme)

	result.Code = CodeOk
}

func main() {
	router := mapRoutes()
	server := &http.Server{
		Addr:    "0.0.0.0:1123",
		Handler: router,
	}

	logger.Infof("octocat is running")
	server.ListenAndServe()
}

func updateREADME(user map[string]interface{}, name, readme string) {
	ak := user["ak"].(string)
	owner := user["login"].(string)

	result := map[string]interface{}{}
	response, bytes, errors := gorequest.New().Get("https://api.github.com/repos/" + owner + "/" + name + "/contents/README.md?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update README failed: %s", errors[0])

		return
	}
	if http.StatusOK != response.StatusCode && http.StatusNotFound != response.StatusCode {
		logger.Errorf("update README status code: %d, body: %s", response.StatusCode, string(bytes))

		return
	}

	content := base64.StdEncoding.EncodeToString([]byte(readme))
	body := map[string]interface{}{
		"message": ":memo: 更新博客",
		"content": content,
	}
	if http.StatusOK == response.StatusCode {
		body["sha"] = result["sha"]
	}

	response, bytes, errors = gorequest.New().Put("https://api.github.com/repos/" + owner + "/" + name + "/contents/README.md?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update README failed: %s", errors[0])

		return
	}
	if http.StatusCreated != response.StatusCode && http.StatusUnprocessableEntity != response.StatusCode {
		logger.Errorf("update README status code: %d, body: %s", response.StatusCode, string(bytes))

		return
	}
}

func createOrUpdateRepo(user map[string]interface{}, name, description, homepage string) (repo map[string]interface{}) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
		"homepage":    homepage,
		"has_wiki":    false,
	}

	ak := user["ak"].(string)

	response, bytes, errors := gorequest.New().Post("https://api.github.com/user/repos?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("create repo failed: %v", errors[0])

		return nil
	}
	if http.StatusCreated != response.StatusCode && http.StatusUnprocessableEntity != response.StatusCode {
		logger.Errorf("create repo status code: %d, body: %s", response.StatusCode, string(bytes))

		return nil
	}
	if http.StatusCreated == response.StatusCode {
		logger.Infof("created repo [%+v]", repo)

		return
	}

	owner := user["login"].(string)
	response, bytes, errors = gorequest.New().Patch("https://api.github.com/repos/" + owner + "/" + name + "?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("create repo failed: %v", errors[0])

		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("create repo status code: %d, body:  %s", response.StatusCode, string(bytes))

		return nil
	}

	logger.Infof("updated repo [%+v]", repo)

	return
}

func user(ak string) (ret map[string]interface{}) {
	response, bytes, errors := gorequest.New().Get("https://api.github.com/user?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).EndStruct(&ret)
	if nil != errors {
		logger.Errorf("get user failed: %v", errors[0])

		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("get user status code: %d, body:  %s", response.StatusCode, string(bytes))

		return nil
	}

	return
}

func randStr(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	time.Sleep(time.Nanosecond)

	letter := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}

	return string(b)
}

func substrBetween(str, open, close string) string {
	s := strings.Index(str, open)
	if -1 == s {
		return ""
	}
	if -1 == strings.Index(str, close) {
		return ""
	}

	s += len(open)
	ret := str[s:]

	return ret[:strings.Index(ret, close)]
}
