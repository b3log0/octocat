// Octocat - B3log 的 GitHub 仓库操作服务
// Copyright (c) 2019-present, b3log.org
//
// Lute is licensed under the Mulan PSL v1.
// You can use this software according to the terms and conditions of the Mulan PSL v1.
// You may obtain a copy of Mulan PSL v1 at:
//     http://license.coscl.org.cn/MulanPSL
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v1 for more details.

package main

import (
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/b3log/gulu"
	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
	"github.com/parnurzeal/gorequest"
)

var blogs = &sync.Map{} // "88250/solo-blog" -> *blog

type blog struct {
	title    string // D 的个人博客 - 开源程序员，自由职业者
	homepage string // https://88250.b3log.org
	repo     string // 88250/solo-blog
	favicon  string
	stat     string
}

var orgAk = ""

var period = time.Hour * 6

func updateAwesomeSolo() {
	if 1 > len(orgAk) {
		return
	}

	for range time.Tick(period) {
		updateAwesomeSoloNow()
	}
}

func updateAwesomeSoloNow() {
	ok, blogCount := updateAwesomeSoloReadme()
	if ok {
		updateAwesomeSoloRepo(blogCount)
	}
}

func updateAwesomeSoloRepo(blogCount int) (repo map[string]interface{}) {
	gulu.Panic.Recover(nil)

	body := map[string]interface{}{
		"name":        "awesome-solo",
		"description": "🎸 展示大家漂亮的 Solo 博客！",
		"has_wiki":    false,
		"has_issues":  true,
	}

	response, bytes, errors := gorequest.New().Patch("https://api.github.com/repos/b3log/awesome-solo?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("update repo failed: %v", errors[0])
		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("update repo [b3log/awesome-solo] status code [%d], body [%s]", response.StatusCode, string(bytes))
		return nil
	}

	logger.Infof("updated repo [b3log/awesome-solo]")
	return
}

func updateAwesomeSoloReadme() (ok bool, blogCount int) {
	gulu.Panic.Recover(nil)

	result := map[string]interface{}{}
	filePath := "README.md"
	content := "| 图标 | 标题 | 链接 | 仓库 |\n"
	content += "| :---: | --- | --- | :---: |\n"
	blogs.Range(func(key, value interface{}) bool {
		blog := value.(*blog)
		title := blog.title
		document, err := goquery.NewDocumentFromReader(strings.NewReader(title))
		if nil == err {
			title = document.Text()
		}
		title = sanitize(title)
		runes := []rune(title)
		if 32 <= len(runes) {
			title = string(runes[:26])
		}
		title = strings.TrimSpace(title)
		if strings.HasSuffix(title, "-") {
			title = title[:len(title)-1]
			title = strings.TrimSpace(title)
		}
		if 1 > len(title) {
			return true
		}
		homepage := sanitize(blog.homepage)
		favicon := sanitize(blog.favicon)
		if 0 < len(favicon) {
			favicon = "<img src=\"" + favicon + "\" width=\"24px\"/>"
			favicon = strings.ReplaceAll(favicon, "/interlace/0", "")
		}
		if strings.Contains(favicon, "solo-") {
			favicon = ""
		}
		content += "| " + favicon + " | " + title + " | " + homepage + " | [:octocat:](https://github.com/" + blog.repo + ") |\n"
		blogCount++
		return true
	})

	if 1 > blogCount {
		return
	}

	content += "\n注：\n\n"
	content += "* 排列顺序是随机的\n"
	content += "* 通过 [Octocat](https://github.com/b3log/octocat) 自动定时刷新，请勿 PR"

	logger.Info("[awesome-solo]'s README.md content is [" + content + "]")

	response, bytes, errors := gorequest.New().Get("https://api.github.com/repos/b3log/awesome-solo/git/trees/master?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(30 * time.Second).EndStruct(&result)
	if nil != errors {
		logger.Errorf("get git tree of file [%s] failed: %s", filePath, errors[0])

		return
	}
	if http.StatusOK != response.StatusCode && http.StatusConflict != response.StatusCode {
		logger.Errorf("get git tree of file [%s] status code [%d], body [%s]", filePath, response.StatusCode, string(bytes))

		return
	}

	now := time.Now()
	pattern := "2006-01-02 15:04:05"
	body := map[string]interface{}{
		"message": ":memo: 定时更新 " + now.Format(pattern) + "，下次刷新时间为 " + now.Add(period).Format(pattern),
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
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

	response, bytes, errors = gorequest.New().Put("https://api.github.com/repos/b3log/awesome-solo/contents/"+filePath+"?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(2 * time.Minute).
		SendMap(body).EndStruct(&result)
	if nil != errors {
		logger.Errorf("update repo [b3log/awesome-solo] file [%s] failed: %s", filePath, errors[0])
		return
	}
	if http.StatusOK != response.StatusCode && http.StatusCreated != response.StatusCode {
		logger.Errorf("update repo [b3log/awesome-solo] file [%s] status code: %d, body: %s", filePath, response.StatusCode, string(bytes))
		return
	}

	logger.Infof("updated repo [b3log/awesome-solo] file [%s]", filePath)
	ok = true
	return
}

func refreshAwesomeSolo(c *gin.Context) {
	updateAwesomeSoloNow()
	c.Status(200)
}

func sanitize(str string) (ret string) {
	ret = bluemonday.UGCPolicy().Sanitize(str)
	ret = strings.ReplaceAll(ret, "\n", " ")
	ret = strings.ReplaceAll(ret, "|", "\\|")
	ret = strings.TrimSpace(ret)
	return
}
