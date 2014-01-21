package main

import (
    "code.google.com/p/go.net/websocket"
    "flag"
    "github.com/codegangsta/martini"
    "github.com/codegangsta/martini-contrib/render"
)

var conf *string = flag.String("-conf", "app.json", "config file name")

func init() {
    flag.Parse()
    DefaultControlCenter = new(ControlCenter)
    DefaultControlCenter.videos = make(map[int]*Video)
    err := initConfig(*conf)
    if err != nil {
        panic(err)
    }
}

func main() {
    m := martini.New()

    m.Use(PanicRecover())
    m.Use(render.Renderer())

    // static files
    m.Use(martini.Static("./static"))

    // add routers
    router.Get("/api/tasks", TasksStatus)
    router.Post("/api/tasks", TasksNew)
    router.Delete("/api/tasks", TasksArchive)
    router.Get("/api/tasks/:id", GetTask(), TaskStatus)
    router.Post("/api/tasks/:id", GetTask(), TaskStart)
    router.Put("/api/tasks/:id", GetTask(), TaskCancel)
    router.Delete("/api/tasks/:id", GetTask(), TaskDelete)
    router.Get("/api/push", websocket.Handler(Push).ServeHTTP)
    router.Get("/api/monitor", websocket.Handler(Monitor).ServeHTTP)
    m.Action(router.Handle)
    m.Run()
}
