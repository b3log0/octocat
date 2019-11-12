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
	"sync"
	"time"

	"github.com/microcosm-cc/bluemonday"

	"github.com/parnurzeal/gorequest"
)

var blogs = &sync.Map{} // "88250/solo-blog" -> *blog

type blog struct {
	title    string // D 的个人博客 - 开源程序员，自由职业者
	homepage string // https://88250.b3log.org
	repo     string // 88250/solo-blog
}

var orgAk = ""

var period = time.Minute * 10

func updateAwesomeSolo() {
	if 1 > len(orgAk) {
		return
	}

	for range time.Tick(period) {
		updateAwesomeSoloRepo()
		updateAwesomeSoloReadme()
	}
}

func updateAwesomeSoloRepo() (repo map[string]interface{}) {
	body := map[string]interface{}{
		"name":        "awesome-solo",
		"description": "展示大家漂亮的 Solo 博客！",
		"has_wiki":    false,
		"has_issues":  false,
		"private":     true,
	}

	response, bytes, errors := gorequest.New().Patch("https://api.github.com/repos/b3log/awesome-solo?access_token="+orgAk).
		Set("User-Agent", UserAgent).Timeout(5 * time.Second).
		SendMap(body).EndStruct(&repo)
	if nil != errors {
		logger.Errorf("create repo failed: %v", errors[0])

		return nil
	}
	if http.StatusOK != response.StatusCode {
		logger.Errorf("create repo [b3log/awesome-solo] status code [%d], body [%s]", response.StatusCode, string(bytes))

		return nil
	}

	logger.Infof("updated repo [b3log/awesome-solo]")

	return
}

func updateAwesomeSoloReadme() (ok bool) {
	result := map[string]interface{}{}
	filePath := "README.md"

	content := ""
	blogs.Range(func(key, value interface{}) bool {
		blog := value.(*blog)
		content += "* [" + blog.title + "](" + blog.homepage + ")  [:octocat:](https://github.com/" + blog.repo + ")\n"

		return true
	})

	if 1 > len(content) {
		return
	}

	content = bluemonday.UGCPolicy().Sanitize(content)

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

	return true
}
