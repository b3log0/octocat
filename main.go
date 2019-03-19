package main

import (
	"encoding/base64"
	"io/ioutil"
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

	f, err := file.Open()
	if nil != err {
		result.Msg = "read file failed"
		c.JSON(http.StatusOK, result)

		return
	}
	fileData, err := ioutil.ReadAll(f)
	if nil != err {
		result.Msg = "read file data failed"
		c.JSON(http.StatusOK, result)

		return
	}

	repoName := c.PostForm("repoName")
	repoReadme := c.PostForm("repoReadme")
	repoDesc := c.PostForm("repoDesc")
	repoHomepage := c.PostForm("repoHomepage")
	toc := c.PostForm("toc")

	repo := createOrUpdateRepo(user, repoName, repoDesc, repoHomepage)
	if nil == repo {
		result.Msg = "create or update repo failed"
		c.JSON(http.StatusOK, result)

		return
	}

	repoFullName := user["login"].(string) + "/" + repoName
	repoReadme = strings.Replace(repoReadme, "${repoFullName}", repoFullName, -1)
	repoReadme = strings.Replace(repoReadme, "${toc}", toc, -1)

	ok := updateFile(user, repoName, "README.md", []byte(repoReadme))
	if ok {
		ok = updateFile(user, repoName, "backup.zip", fileData)
	}
	if ok {
		result.Code = CodeOk
	}

	c.JSON(http.StatusOK, result)
}

func updateFile(user map[string]interface{}, repoName, filePath string, content [] byte) (ok bool) {
	ak := user["ak"].(string)
	owner := user["login"].(string)

	result := map[string]interface{}{}
	response, bytes, errors := gorequest.New().Get("https://api.github.com/repos/" + owner + "/" + repoName + "/git/trees/master?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(30 * time.Second).EndStruct(&result)
	if nil != errors {
		logger.Errorf("get git tree of file [%s] failed: %s", filePath, errors[0])

		return
	}
	if http.StatusOK != response.StatusCode && http.StatusConflict != response.StatusCode {
		logger.Errorf("get git tree of file [%s] status code: %d, body: %s", filePath, response.StatusCode, string(bytes))

		return
	}

	body := map[string]interface{}{
		"message": ":memo: 更新博客",
		"content": base64.StdEncoding.EncodeToString(content),
	}
	if http.StatusOK == response.StatusCode {
		tree := result["tree"].([]interface{})
		for _, f := range tree {
			file := f.(map[string]interface{})
			if filePath == file["path"].(string) {
				body["sha"] = file["sha"]
				break
			}
		}
	}

	response, bytes, errors = gorequest.New().Put("https://api.github.com/repos/" + owner + "/" + repoName + "/contents/" + filePath + "?access_token=" + ak).
		Set("User-Agent", UserAgent).Timeout(2 * time.Minute).
		SendMap(body).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update file [%s] failed: %s", filePath, errors[0])

		return
	}
	if http.StatusOK != response.StatusCode && http.StatusCreated != response.StatusCode {
		logger.Errorf("update file [%s] status code: %d, body: %s", filePath, response.StatusCode, string(bytes))

		return
	}

	logger.Infof("updated file [%s] in repo [%s]", filePath, owner+"/"+repoName)

	return true
}

func createOrUpdateRepo(user map[string]interface{}, repoName, repoDesc, repoHomepage string) (repo map[string]interface{}) {
	body := map[string]interface{}{
		"name":        repoName,
		"description": repoDesc,
		"homepage":    repoHomepage,
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
	response, bytes, errors = gorequest.New().Patch("https://api.github.com/repos/" + owner + "/" + repoName + "?access_token=" + ak).
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

	logger.Infof("updated repo [%v]", repo)

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

func main() {
	router := mapRoutes()
	server := &http.Server{
		Addr:    "0.0.0.0:1123",
		Handler: router,
	}

	logger.Infof("octocat is running")
	server.ListenAndServe()
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
