package main

import (
    "github.com/tuxychandru/pubsub"
)

var NewsRoom = pubsub.New(16)

const (
    TOPIC_NEW      string = "new"
    TOPIC_FAIL     string = "fail"
    TOPIC_CANCEL   string = "cancel"
    TOPIC_SUCCESS  string = "success"
    TOPIC_DOWNLOAD string = "download"
    TOPIC_COMBINE  string = "combine"
    TOPIC_ERROR    string = "error"
    TOPIC_DELETE   string = "delete"
    TOPIC_WAIT     string = "wait"
)
