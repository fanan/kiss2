package main

import (
    "fmt"
    "github.com/codegangsta/martini"
    "github.com/codegangsta/martini-contrib/render"
    "io/ioutil"
    "net/http"
    "strconv"
)

func PanicRecover() martini.Handler {
    return func(c martini.Context, w http.ResponseWriter) {
        defer func() {
            err := recover()
            if err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                fmt.Fprint(w, err)
            }
        }()
        c.Next()
    }
}

var router = martini.NewRouter()

func TasksStatus(r render.Render) {
    info := DefaultControlCenter.Status()
    r.JSON(http.StatusOK, info)
}

func TasksNew(req *http.Request, r render.Render) {
    defer req.Body.Close()
    c, err := ioutil.ReadAll(req.Body)
    if err != nil {
        panic(err)
    }
    v, err := DefaultControlCenter.New(string(c))
    if err != nil {
        panic(err)
    }
    r.JSON(http.StatusOK, v.Info())
}

func TasksArchive(r render.Render) {
    DefaultControlCenter.Archive()
    r.JSON(http.StatusOK, "ok")
}

func GetTask() martini.Handler {
    return func(c martini.Context, params martini.Params) {
        id, err := strconv.Atoi(params["id"])
        if err != nil {
            panic(err)
        }
        v, ok := DefaultControlCenter.Get(id)
        if !ok || v == nil {
            panic(ErrVideoNotFound)
        }
        c.Map(v)
        c.Next()
    }
}

func TaskStatus(v *Video, r render.Render) {
    if v.Status == StatusDownloading {
        pg, _ := v.Progress()
        var msg string
        if v.err != nil {
            msg = v.err.Error()
        }
        r.JSON(http.StatusOK, VideoProgress{Status: v.Status, Url: v.Url, Id: v.Id, Name: v.Name, Err: msg, Total: pg.Total, Finished: pg.Finished})
    } else {
        r.JSON(http.StatusOK, v.Info())
    }
}

func TaskCancel(v *Video, r render.Render) {
    err := v.Cancel()
    if err != nil {
        panic(err)
    }
    r.JSON(http.StatusOK, "ok")
}

func TaskStart(v *Video, r render.Render) {
    switch v.Status {
    case StatusFailure, StatusUnstarted:
        go v.Do()
        r.JSON(http.StatusOK, "ok")
    default:
        panic(ErrCannotStart)
    }
}

func TaskDelete(v *Video, r render.Render) {
    DefaultControlCenter.Delete(v.Id)
    r.JSON(http.StatusOK, "ok")
}
