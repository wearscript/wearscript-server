package main

import (
	"fmt"
	"github.com/wearscript/wearscript-go/wearscript"
)


func WeariverseGistHandle(cm *wearscript.ConnectionManager, userId string, request []interface{}) {
	action := request[1].(string)
	channelResult := request[2].(string)
	fmt.Println("gist action: " + action + " result: " + channelResult)
	var dataJS interface{}
	var err error
	if action == "list" {
		dataJS, err = WeariverseGetGists(userId)
	} else {
		dataJS = "error:action"
	}
	if err != nil {
		fmt.Println(err)
	}
	cm.Publish(channelResult, dataJS)
}

func WeariverseGetGists(userId string) (interface{}, error) {
	//[]string{"9732959", "9423246"}
	gists, err := getUserSortedSet("weariverse", "gists")
	if err != nil {
		return nil, err
	}
	out := []interface{}{}
	for _, value := range gists {
		data, err := GithubGetGist(userId, value)
		if err != nil {
			fmt.Println("weariverse: couldn't get gist")
			continue
		}
		out = append(out, data)
	}
	return out, nil
}
