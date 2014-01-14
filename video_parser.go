package main

type VideoParser interface {
    Parse() error
    SetOwner(*Video)
    GetOwner() *Video
}
