package main

import (
    "bufio"
    "crypto/md5"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "net/http"
    "strconv"
    "strings"
)

const (
    iqiyi_vid_pattern       string = "data-player-videoid"
    iqiyi_tvid_pattern      string = "data-player-tvid"
    iqiyi_name_pattern      string = "http://cache.video.qiyi.com/vi/%s/%s/"
    iqiyi_info_pattern      string = "http://cache.video.qiyi.com/vp/%s/%s/"
    iqiyi_time_url          string = "http://data.video.qiyi.com/v.f4v"
    iqiyi_clip_info_pattern string = "http://data.video.qiyi.com/%s/videos%s"
    iqiyi_md5_pattern       string = `)(*&^flash@#$%a`
)

const (
    QUALITY_4K         int = 10
    QUALITY_FULL_HD    int = 5
    QUALITY_SUPER_HIGH int = 4
    QUALITY_SUPER      int = 3
    QUALITY_HIGH       int = 2
    QUALITY_STANDARD   int = 1
    QUALITY_TOPSPEED   int = 96
    QUALITY_NONE       int = 0
)

var (
    iqiyi_qualities []int = []int{QUALITY_4K, QUALITY_FULL_HD, QUALITY_SUPER_HIGH, QUALITY_SUPER, QUALITY_HIGH, QUALITY_STANDARD, QUALITY_TOPSPEED}
)

type IQiYiVideoParser struct {
    owner *Video
    vid   string   //video id
    tvid  string   //tv id
    aid   string   //album id
    info  *IqyInfo // info
}

func (self *IQiYiVideoParser) SetOwner(v *Video) {
    self.owner = v
}

func (self *IQiYiVideoParser) GetOwner() (v *Video) {
    return self.owner
}

func (self *IQiYiVideoParser) parseVidAndTVid() error {
    resp, err := http.Get(self.owner.Url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    br := bufio.NewReader(resp.Body)
    var foundVid, foundTVid bool
    for {
        line, err := br.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return err
        }
        if !foundVid {
            if strings.Contains(line, iqiyi_vid_pattern) {
                items := strings.SplitN(line, "\"", 3)
                if len(items) != 3 {
                    return ErrParseBadFormat
                }
                self.vid = items[1]
                foundVid = true
            }
        }
        if !foundTVid {
            if strings.Contains(line, iqiyi_tvid_pattern) {
                items := strings.SplitN(line, "\"", 3)
                if len(items) != 3 {
                    return ErrParseBadFormat
                }
                self.tvid = items[1]
                foundTVid = true
            }
        }
        if foundVid && foundTVid {
            break
        }
    }
    if foundVid && foundTVid {
        return nil
    }
    return ErrParseBadFormat
}

type iQiYiName struct {
    Aid  int    `json:"aid"`
    Name string `json:"shortTitle"`
}

func (self *IQiYiVideoParser) parseName() (err error) {
    url := fmt.Sprintf(iqiyi_name_pattern, self.tvid, self.vid)
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    c, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    iqyName := new(iQiYiName)
    err = json.Unmarshal(c, iqyName)
    if err != nil {
        return err
    }
    self.owner.Name = iqyName.Name
    self.aid = strconv.Itoa(iqyName.Aid)
    return nil
}

type iqyClipInfo struct {
    Msz      int    `json:"msz"`
    Size     int64  `json:"b"`
    Duration int64  `json:"d"`
    Location string `json:"l"`
}

type iqyVideo struct {
    Bid  int           `json:"bid"`
    Fs   []iqyClipInfo `json:"fs"`
    Flvs []iqyClipInfo `json:"flvs"`
}

type iqyVs struct {
    Vs []iqyVideo `json:"vs"`
}

type IqyInfo struct {
    Dd  string  `json:"dd"`
    Tkl []iqyVs `json:"tkl"`
}

func (self *IQiYiVideoParser) parseInfo() (err error) {
    url := fmt.Sprintf(iqiyi_info_pattern, self.tvid, self.vid)
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    c, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    self.info = new(IqyInfo)
    err = json.Unmarshal(c, self.info)
    if err != nil {
        return err
    }
    return nil
}

func (self *IQiYiVideoParser) parseClips() (err error) {
    if len(self.info.Tkl) != 1 {
        return ErrParseBadFormat
    }
    n := len(self.info.Tkl[0].Vs)
    if n < 1 {
        return ErrParseBadFormat
    }
    //fuck quality order 10 > 5 > 4 > 3 > 2 > 1 > 96 > 0
    var idx int
    //var v iqyVideo
    var found bool
quality_loop:
    for _, quality := range iqiyi_qualities {
        for idx, _ = range self.info.Tkl[0].Vs {
            if self.info.Tkl[0].Vs[idx].Bid == quality {
                found = true
                break quality_loop
            }
        }
    }
    if !found {
        return ErrParseBadFormat
    }
    self.owner.logger.Printf("%+v", self.info.Tkl[0].Vs[idx])
    v := self.info.Tkl[0].Vs[idx]
    n1, n2 := len(v.Fs), len(v.Flvs)
    if n1 == 0 && n2 == 0 {
        return ErrParseBadFormat
    }
    var advHandle bool
    switch self.info.Tkl[0].Vs[idx].Bid {
    case QUALITY_4K, QUALITY_FULL_HD, QUALITY_SUPER_HIGH:
        advHandle = true
    default:
        advHandle = false
    }
    var vs []iqyClipInfo
    if n1 == 0 {
        vs = v.Flvs
        n = n2
    } else {
        vs = v.Fs
        n = n1
    }
    self.owner.nClips = n
    self.owner.clips = make([]*Clip, 0, self.owner.nClips)
    for i := 0; i < n; i++ {
        clip := new(Clip)
        clip.logger = self.owner.logger
        clip.total = vs[i].Size
        var ci iqyClipParserInfo = iqyClipParserInfo{dd: self.info.Dd}
        if advHandle {
            ci.location, err = iqyDecodeLoation(vs[i].Location)
            if err != nil {
                return err
            }
        } else {
            ci.location = vs[i].Location
        }
        self.owner.logger.Printf(ci.location)
        clip.SetInfo(ci)
        clip.parser = new(IqyClipParser)
        clip.parser.SetOwner(clip)
        self.owner.clips = append(self.owner.clips, clip)
    }
    return nil
}

func iqyDecodeLoation(l string) (string, error) {
    items := strings.Split(l, "-")
    n := len(items)
    //i := n - 1
    b := make([]byte, n, n)
    for i := n - 1; i >= 0; i-- {
        ui1, err := strconv.ParseInt(items[n-i-1], 16, 64)
        if err != nil {
            return "", err
        }
        var c byte
        switch i % 3 {
        case 2:
            c = byte(ui1 ^ 72)
        case 1:
            c = byte(ui1 ^ 121)
        default:
            c = byte(ui1 ^ 103)
        }
        b[i] = c
    }
    return string(b), nil
}

type iqyClipParserInfo struct {
    location string
    dd       string
}

type IqyClipParser struct {
    owner *Clip
}

func (self *IqyClipParser) SetOwner(c *Clip) {
    self.owner = c
}

func (self *IqyClipParser) GetOwner() (c *Clip) {
    return self.owner
}

type iqyClipRealUrl struct {
    L string `json:"l"`
}

func (self *IqyClipParser) parse() (url string, err error) {
    info, ok := self.owner.info.(iqyClipParserInfo)
    if !ok {
        return "", ErrParseBadFormat
    }
    key, err := self.getKey(info.location)
    if err != nil {
        return "", err
    }
    reqUrl := fmt.Sprintf(iqiyi_clip_info_pattern, key, info.location)
    self.owner.logger.Println(reqUrl)
    resp, err := http.Get(reqUrl)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    c, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    realInfo := new(iqyClipRealUrl)
    err = json.Unmarshal(c, realInfo)
    if err != nil {
        return "", err
    }
    return realInfo.L, err
}

type iqyTimeInfo struct {
    T string `json:"time"`
}

func (self *IqyClipParser) getKey(location string) (key string, err error) {
    self.owner.logger.Printf(location)
    items := strings.Split(location, "/")
    n := len(items)
    if n < 2 {
        return "", ErrParseBadFormat
    }
    ss := strings.Split(items[n-1], ".")
    if len(ss) != 2 {
        return "", ErrParseBadFormat
    }
    location = ss[0]
    resp, err := http.Get(iqiyi_time_url)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    c, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    t := new(iqyTimeInfo)
    err = json.Unmarshal(c, t)
    if err != nil {
        return "", err
    }
    ti, err := strconv.Atoi(t.T)
    if err != nil {
        return "", err
    }
    b := []byte(strconv.Itoa(ti / 600))
    b = append(b, []byte(iqiyi_md5_pattern)...)
    b = append(b, []byte(location)...)
    h := md5.New()
    _, err = h.Write(b)
    if err != nil {
        return "", err
    }
    ret := fmt.Sprintf("%x", h.Sum(nil))
    return ret, nil
}

func (self *IqyClipParser) Parse() (url string, err error) {
    for i := 0; i < Lives; i++ {
        url, err = self.parse()
        if err != nil {
            continue
        }
        return url, nil
    }
    return url, err
}
