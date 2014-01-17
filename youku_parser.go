package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/url"
    "strconv"
    "strings"
)

const (
    youku_basic_info_url       string = "http://v.youku.com/player/getPlayList/VideoIDS/%s"
    youku_encrypt_source       string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ/\\:._-1234567890"
    youku_download_url_pattern string = "http://f.youku.com/player/getFlvPath/sid/00_%02X/st/%s/fileid/%s%02X%s?K=%s"
)

var formatValue map[string]int = map[string]int{
    "flv": 1,
    "mp4": 2,
    "hd2": 3,
    "hd3": 4,
}

type YoukuVideoParser struct {
    vid     string
    fileids string
    owner   *Video
    info    *YoukuData
    segs    []YoukuClipInfo
    format  string
}

type YoukuData struct {
    Seed  int    `json:"seed"`
    Vid   string `json:"vidEncoded"`
    Title string `json:"title"`
    //Key1            string                     `json:"key1"`
    //Key2            string                     `json:"key2"`
    StreamFileIds   map[string]string          `json:"streamfileids"`
    StreamFileTypes []string                   `json:"streamtypes"`
    Segs            map[string][]YoukuClipInfo `json:"segs"`
}

type YoukuInfo struct {
    Data [1]*YoukuData `json:"data"`
}

type YoukuClipInfo struct {
    //No   string `json:"no"`
    Size string `json:"size"`
    K    string `json:"k"`
    //K2   string `json:"k2"`
}

func (self *YoukuVideoParser) SetOwner(v *Video) {
    self.owner = v
}

func (self *YoukuVideoParser) GetOwner() *Video {
    return self.owner
}

func (self *YoukuVideoParser) parseVid() error {
    u, err := url.Parse(self.owner.Url)
    if err != nil {
        return err
    }
    items := strings.SplitN(u.Path, "_", 3)
    if len(items) != 3 {
        return ErrParseBadFormat
    }
    self.vid = strings.TrimSuffix(items[2], ".html")
    return nil
}

func (self *YoukuVideoParser) parseBasicInfo() error {
    resp, err := http.Get(fmt.Sprintf(youku_basic_info_url, self.vid))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    c, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    info := new(YoukuInfo)
    self.owner.logger.Printf("received json: %s", string(c))
    err = json.Unmarshal(c, info)
    if err != nil {
        return err
    }
    self.info = info.Data[0]
    if self.info == nil {
        return ErrParseBadFormat
    }
    self.owner.Name = self.info.Title
    return nil
}

func (self *YoukuVideoParser) parseVideoTypes() error {
    var value int = 0
    var format string
    for _, f := range self.info.StreamFileTypes {
        v := formatValue[f]
        if v > value {
            format = f
            value = v
        }
    }
    var ok bool
    self.fileids, ok = self.info.StreamFileIds[format]
    if !ok {
        return ErrParseBadFormat
    }
    self.segs, ok = self.info.Segs[format]
    if !ok {
        return ErrParseBadFormat
    }
    if format == "mp4" {
        self.format = "mp4"
    } else {
        self.format = "flv"
    }
    return nil
}

func (self *YoukuVideoParser) parseClips() error {
    var err error
    n := len(self.segs)
    if n == 0 {
        return ErrParseBadFormat
    }
    self.owner.clips = make([]*Clip, 0, n)
    m := getFileIDMixString(self.info.Seed)
    items := strings.Split(strings.TrimSuffix(self.fileids, "*"), "*")
    b := make([]byte, len(items), len(items))
    for i, v := range items {
        vv, err := strconv.Atoi(v)
        if err != nil {
            return err
        }
        b[i] = m[vv]
    }
    if len(b) < 10 {
        return ErrParseBadFormat
    }
    fileid1 := string(b[0:8])
    fileid2 := string(b[10:])
    self.owner.nClips = len(self.segs)
    self.owner.clips = make([]*Clip, 0, self.owner.nClips)
    for idx, seg := range self.segs {
        clip := new(Clip)
        clip.logger = self.owner.logger
        clip.total, err = strconv.ParseInt(seg.Size, 10, 64)
        if err != nil {
            return err
        }
        clip.fetchUrl = fmt.Sprintf(youku_download_url_pattern, idx, self.format, fileid1, idx, fileid2, seg.K)
        clip.parser = new(YoukuClipParser)
        clip.parser.SetOwner(clip)
        self.owner.clips = append(self.owner.clips, clip)
    }
    return nil
}

func (self *YoukuVideoParser) Parse() (err error) {
    for i := 0; i < Lives; i++ {
        err = self.parseVid()
        if err != nil {
            return err
        }
        err = self.parseBasicInfo()
        if err != nil {
            return err
        }
        err = self.parseVideoTypes()
        if err != nil {
            return err
        }
        err = self.parseClips()
        if err != nil {
            return err
        }
        return nil
    }
    return err
}

func getFileIDMixString(seed int) []byte {
    l := len(youku_encrypt_source)
    index := make([]int, l, l)
    for i := 0; i < l; i++ {
        seed = (seed*211 + 30031) % 65536
        index[i] = seed * (l - i) / 65536
    }
    b := []byte(youku_encrypt_source)
    m := make([]byte, l, l)
    for j, idx := range index {
        i := 0
        met := 0
    inner:
        for {
            if b[i] == 0 {
                i++
                continue
            }
            if met == idx {
                m[j] = b[i]
                b[i] = 0
                break inner
            }
            met++
            i++
        }
    }
    return m
}

type YoukuClipParser struct {
    owner *Clip
}

func (self *YoukuClipParser) SetOwner(c *Clip) {
    self.owner = c
}

func (self *YoukuClipParser) GetOwner() *Clip {
    return self.owner
}

func (self *YoukuClipParser) Parse() (string, error) {
    return self.owner.fetchUrl, nil
}
