package main

import (
	"bytes"
	"code.google.com/p/goauth2/oauth"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"strconv"
	"github.com/wearscript/wearscript-go/wearscript"
)

func AuthHandlerGH(w http.ResponseWriter, r *http.Request) {
	userId, err := userID(r)
	if userId == "" || err != nil {
		fmt.Println("User id not found")
		http.Redirect(w, r, fullUrl+"/auth", http.StatusFound)
		return
	}
	rand, err := RandString()
	if err != nil {
		w.WriteHeader(500)
		LogPrintf("github: random")
		return
	}
	url := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&scope=gist&state=%s&redirect_uri=%s/oauth2callbackgh", clientIdGH, UB64Enc(rand), fullUrl)
	http.Redirect(w, r, url, http.StatusFound)
}

func configgh() *oauth.Config {
	r := &oauth.Config{
		ClientId:     clientIdGH,
		ClientSecret: clientSecretGH,
		Scope:        "gist",
		TokenURL:     "https://github.com/login/oauth/access_token",
		RedirectURL:  fullUrl + "/oauth2callbackgh",
	}
	return r
}

func Oauth2callbackHandlerGH(w http.ResponseWriter, r *http.Request) {
	userId, err := userID(r)
	if userId == "" || err != nil {
		http.Redirect(w, r, fullUrl+"/auth", http.StatusFound)
		return
	}
	t := &oauth.Transport{Config: configgh()}
	// Exchange the code for access and refresh tokens.
	tok, err := t.Exchange(r.FormValue("code"))
	if err != nil {
		w.WriteHeader(500)
		LogPrintf("oauthgh: exchange")
		return
	}
	err = setUserAttribute(userId, "oauth_token_gh", tok.AccessToken)
	if err != nil {
		w.WriteHeader(500)
		LogPrintf("oauthgh: store token")
		return
	}
	fmt.Println(tok)

	http.Redirect(w, r, fullUrl, http.StatusFound)
}

func githubConvertFiles(filesRaw map[interface{}]interface{}) map[string]map[string]string {
	files := map[string]map[string]string{}
	for k0, v0 := range filesRaw {
		fileRaw := v0.(map[interface{}]interface{})
		file := map[string]string{}
		for k1, v1 := range fileRaw {
			file[k1.(string)] = string(v1.([]uint8))
		}
		files[k0.(string)] = file
	}
	return files

}

func GithubGistHandle(cm *wearscript.ConnectionManager, userId string, request []interface{}) {
	action := request[1].(string)
	channelResult := request[2].(string)
	fmt.Println("gist action: " + action + " result: " + channelResult)
	var dataJS interface{}
	var err error
	if action == "list" {
		dataJS, err = GithubGetGists(userId)
	} else if action == "get" {
		dataJS, err = GithubGetGist(userId, request[3].(string))
	} else if action == "create" {
		dataJS, err = GithubCreateGist(userId, request[3].(bool), request[4], githubConvertFiles(request[5].(map[interface{}]interface{})))
	} else if action == "modify" {
		dataJS, err = GithubModifyGist(userId, request[3].(string), request[4], githubConvertFiles(request[5].(map[interface{}]interface{})))
	} else if action == "fork" {
		dataJS, err = GithubForkGist(userId, request[3].(string))
	} else {
		dataJS = "error:action"
	}
	if err != nil {
		fmt.Println(err)
	}
	cm.Publish(channelResult, dataJS)
}

func GithubGetGists(userId string) (interface{}, error) {
	values := url.Values{}
	accessToken, err := getUserAttribute(userId, "oauth_token_gh")
	if err != nil {
		return "error:oauth", err
	}
	values.Set("access_token", accessToken)
	r, err := http.Get("https://api.github.com/gists" + "?" + values.Encode())
	if err != nil {
		return "error:github", err
	}
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if r.StatusCode != 200 {
		return "error:status:" + strconv.Itoa(r.StatusCode), errors.New(fmt.Sprintf("Bad status[%d][%s]", r.StatusCode, data))
	}
	datas := []map[string]interface{}{}
	datasKeep := []map[string]interface{}{}
	err = json.Unmarshal(data, &datas)
	if err != nil {
		return "error:json", err
	}
	for _, v := range datas {
		if githubCheckDescription(v) {
			datasKeep = append(datasKeep, v)
		}
	}
	return datasKeep, nil
}

func githubCheckDescription(gist map[string]interface{}) bool {
	vs, ok := gist["description"].(string)
	return ok && strings.HasPrefix(vs, "[wearscript]")
}

func GithubGetGist(userId string, gistId string) (interface{}, error) {
	values := url.Values{}
	accessToken, err := getUserAttribute(userId, "oauth_token_gh")
	if err != nil {
		return "error:oauth", err
	}
	values.Set("access_token", accessToken)
	response, err := http.Get("https://api.github.com/gists/" + gistId + "?" + values.Encode())
	if err != nil {
		return "error:github", err
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "error:github", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "error:status:" + strconv.Itoa(response.StatusCode), errors.New(fmt.Sprintf("Bad status[%d][%s]", response.StatusCode, body))
	}
	datas := map[string]interface{}{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		return "error:github", err
	}
	return datas, nil
}

func GithubCheckGist(userId string, gistId string) bool {
	data, err := GithubGetGist(userId, gistId)
	dataMap, ok := data.(map[string]interface{})
	if err != nil || !ok {
		LogPrintf("github: check gist")
		return false
	}
	return githubCheckDescription(dataMap)
}

func GithubCreateGist(userId string, public bool, description interface{}, files map[string]map[string]string) (interface{}, error) {
	accessToken, err := getUserAttribute(userId, "oauth_token_gh")
	if err != nil {
		return "error:oauth", err
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	data := map[string]interface{}{}
	descriptionStr, ok := description.(string)
	if ok {
		data["description"] = descriptionStr
	}
	data["public"] = public
	data["files"] = files
	datajs, err := json.Marshal(data)
	if err != nil {
		return "error:json", err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.github.com/gists?%s", values.Encode()), bytes.NewBuffer(datajs))
	if err != nil {
		return "error:github", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error:github", err
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return "error:github", err
	}
	if response.StatusCode != 201 {
		return "error:status:" + strconv.Itoa(response.StatusCode), errors.New(fmt.Sprintf("Bad status[%d][%s]", response.StatusCode, body))
	}
	datas := map[string]interface{}{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		return "error:github", err
	}
	return datas, nil
}

func GithubModifyGist(userId string, gistId string, description interface{}, files map[string]map[string]string) (interface{}, error) {
	if !GithubCheckGist(userId, gistId) {
		return "error:check", errors.New("GithubCheckGist failed")
	}
	accessToken, err := getUserAttribute(userId, "oauth_token_gh")
	if err != nil {
		return "error:oauth", err
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	data := map[string]interface{}{}
	descriptionStr, ok := description.(string)
	if ok {
		data["description"] = descriptionStr
	}
	data["files"] = files
	datajs, err := json.Marshal(data)
	if err != nil {
		return "error:json", err
	}
	req, err := http.NewRequest("PATCH", fmt.Sprintf("https://api.github.com/gists/%s?%s", gistId, values.Encode()), bytes.NewBuffer(datajs))
	if err != nil {
		return "error:github", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error:github", err
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return "error:github", err
	}
	if response.StatusCode != 200 {
		return "error:status:" + strconv.Itoa(response.StatusCode), errors.New(fmt.Sprintf("Bad status[%d][%s]", response.StatusCode, body))
	}
	datas := map[string]interface{}{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		return "error:github", err
	}
	return datas, nil
}

func GithubForkGist(userId string, gistId string) (interface{}, error) {
	if !GithubCheckGist(userId, gistId) {
		return "error:check", errors.New("GithubCheckGist failed")
	}
	accessToken, err := getUserAttribute(userId, "oauth_token_gh")
	if err != nil {
		return "error:oauth", err
	}
	values := url.Values{}
	values.Set("access_token", accessToken)
	data := map[string]interface{}{}
	datajs, err := json.Marshal(data)
	if err != nil {
		return "error:oauth", err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.github.com/gists/%s/forks?%s", gistId, values.Encode()), bytes.NewBuffer(datajs))
	if err != nil {
		return "error:github", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error:github", err
	}
	body, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return "error:github", err
	}
	if response.StatusCode != 201 {
		return "error:status:" + strconv.Itoa(response.StatusCode), errors.New(fmt.Sprintf("Bad status[%d][%s]", response.StatusCode, body))
	}
	datas := map[string]interface{}{}
	err = json.Unmarshal(body, &datas)
	if err != nil {
		return "error:github", err
	}
	return datas, nil
}
