package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/*
var assets embed.FS
func Handler()http.Handler{
	sub,err:=fs.Sub(assets,"dist");if err!=nil{return http.NotFoundHandler()};files:=http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){p:=strings.TrimPrefix(path.Clean("/"+r.URL.Path),"/");if p=="."||p==""{p="index.html"};if _,err:=fs.Stat(sub,p);err!=nil{r.URL.Path="/index.html"};files.ServeHTTP(w,r)})
}
