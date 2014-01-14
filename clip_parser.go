package main

type ClipParser interface {
    Parse() (string, error)
    SetOwner(*Clip)
    GetOwner() *Clip
}
