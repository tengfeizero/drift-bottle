package web

import "embed"

// Dist 内嵌的前端构建产物；构建脚本会把 frontend/dist 拷贝到 web/dist 后再编译
//
//go:embed all:dist
var Dist embed.FS
