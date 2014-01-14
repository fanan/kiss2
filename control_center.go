package main

import (
    "encoding/json"
    "errors"
    "os"
    "sync"
)

var Queue chan int

type ControlCenter struct {
    sync.Mutex
    db     string
    videos map[int]*Video
}

var DefaultControlCenter *ControlCenter

func (self *ControlCenter) Get(id int) (*Video, bool) {
    self.Lock()
    defer self.Unlock()
    v, ok := self.videos[id]
    return v, ok
}

func (self *ControlCenter) New(url string) (*Video, error) {
    self.Lock()
    defer self.Unlock()
    var max int
    for id, video := range self.videos {
        if id > max {
            max = id
        }
        if video.Url == url {
            return nil, ErrUrlDuplicated
        }
    }
    max++
    v := &Video{Url: url, Id: max}
    self.videos[max] = v
    go v.Do()
    return v, nil
}

func (self *ControlCenter) Delete(id int) {
    self.Lock()
    defer self.Unlock()
    _, ok := self.videos[id]
    if ok {
        delete(self.videos, id)
    }
    return
}

func (self *ControlCenter) Archive() {
    self.Lock()
    defer self.Unlock()
    for id, video := range self.videos {
        if video.Status == StatusSuccess {
            delete(self.videos, id)
        }
    }
    return
}

type VideoInfo struct {
    Status int    `json:"status"`
    Id     int    `json:"id"`
    Err    string `json:"error"`
    Name   string `json:"name"`
    Url    string `json:"url"`
}

type VideoProgress struct {
    Status   int    `json:"status"`
    Id       int    `json:"id"`
    Err      string `json:"error"`
    Name     string `json:"name"`
    Url      string `json:"url"`
    Total    int64  `json:"total"`
    Finished int64  `json:"finished"`
}

func (self *VideoInfo) Video() *Video {
    var err error
    if self.Err != "" {
        err = errors.New(self.Err)
    }
    return &Video{Status: self.Status, Id: self.Id, err: err, Name: self.Name, Url: self.Url}
}

func (self *ControlCenter) Save() error {
    self.Lock()
    defer self.Unlock()
    info := make([]*VideoInfo, 0, len(self.videos))
    for _, v := range self.videos {
        info = append(info, v.Info())
    }
    fp, err := os.Create(self.db)
    if err != nil {
        return err
    }
    defer fp.Close()
    return json.NewEncoder(fp).Encode(info)
}

func (self *ControlCenter) Init() error {
    self.Lock()
    defer self.Unlock()
    fp, err := os.Open(self.db)
    if err != nil {
        return err
    }
    info := make([]*VideoInfo, 0)
    err = json.NewDecoder(fp).Decode(&info)
    if err != nil {
        return err
    }
    for _, vi := range info {
        self.videos[vi.Id] = vi.Video()
        switch vi.Status {
        case StatusCombining, StatusConverting, StatusDownloading:
            self.videos[vi.Id].Status = StatusFailure
        case StatusWaiting:
            self.videos[vi.Id].Status = StatusUnstarted
        }
    }
    return nil
}

func (self *ControlCenter) Status() (info []interface{}) {
    self.Lock()
    defer self.Unlock()
    info = make([]interface{}, 0, len(self.videos))
    for _, v := range self.videos {
        if v.Status != StatusDownloading {
            info = append(info, v.Info())
        } else {
            pg, _ := v.Progress() // client's duty to handle error
            var msg string
            if v.err != nil {
                msg = v.err.Error()
            }
            info = append(info, &VideoProgress{Status: v.Status, Url: v.Url, Id: v.Id, Name: v.Name, Err: msg, Total: pg.Total, Finished: pg.Finished})
        }
    }
    return info
}
