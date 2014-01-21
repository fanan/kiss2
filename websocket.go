package main

import (
    "code.google.com/p/go.net/websocket"
)

type WsInfo struct {
    Action int         `json:"action"`
    Data   interface{} `json:"data"`
}

const (
    ACTION_ERROR int = iota
    ACTION_DELETE
    ACTION_CHANGE
)

func Monitor(ws *websocket.Conn) {
    chString := NewsRoom.Sub(TOPIC_ERROR)
    chInfo := NewsRoom.Sub(TOPIC_FAIL, TOPIC_SUCCESS)
    defer func() {
        NewsRoom.Unsub(chString, TOPIC_ERROR)
        NewsRoom.Unsub(chInfo, TOPIC_FAIL, TOPIC_SUCCESS)
    }()
    var err error
    for {
        select {
        case iString := <-chString:
            s, ok := iString.(string)
            if !ok {
                break
            }
            err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_ERROR, Data: s})
            if err != nil {
                break
            }
        case iInfo := <-chInfo:
            info, ok := iInfo.(*VideoInfo)
            if !ok {
                break
            }
            if info != nil {
                err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_CHANGE, Data: info})
                if err != nil {
                    break
                }
            }
        }
    }
}

func Push(ws *websocket.Conn) {
    chString := NewsRoom.Sub(TOPIC_ERROR)
    chDownload := NewsRoom.Sub(TOPIC_DOWNLOAD)
    chInfo := NewsRoom.Sub(TOPIC_NEW, TOPIC_FAIL, TOPIC_COMBINE, TOPIC_CANCEL, TOPIC_SUCCESS, TOPIC_WAIT)
    chDelete := NewsRoom.Sub(TOPIC_DELETE)
    defer func() {
        NewsRoom.Unsub(chString, TOPIC_ERROR)
        NewsRoom.Unsub(chDownload, TOPIC_DOWNLOAD)
        NewsRoom.Unsub(chInfo, TOPIC_NEW, TOPIC_FAIL, TOPIC_COMBINE, TOPIC_CANCEL, TOPIC_SUCCESS, TOPIC_WAIT)
        NewsRoom.Unsub(chDelete, TOPIC_DELETE)
    }()

    var err error
    for {
        select {
        case iString := <-chString:
            s, ok := iString.(string)
            if !ok {
                break
            }
            err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_ERROR, Data: s})
            if err != nil {
                break
            }
        case iDown := <-chDownload:
            pg, ok := iDown.(*VideoProgress)
            if !ok {
                break
            }
            if pg != nil {
                err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_CHANGE, Data: pg})
                if err != nil {
                    break
                }
            }
        case iInfo := <-chInfo:
            info, ok := iInfo.(*VideoInfo)
            if !ok {
                break
            }
            if info != nil {
                err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_CHANGE, Data: info})
                if err != nil {
                    break
                }
            }
        case iId := <-chDelete:
            id, ok := iId.(int)
            if !ok {
                break
            }
            err = websocket.JSON.Send(ws, WsInfo{Action: ACTION_DELETE, Data: id})
            if err != nil {
                break
            }
        }
    }
}
