# okcoin-terminal

使用golang 开发的 okcoin 行情 terminal 显示

安装：

```
glide install
```

运行：
```
go run okcoin.go

或者运行可执行文件

./bin/okcoin
```

编译:

```
go build -ldflags "-s -w" -o bin/okcoin okcoin.go

windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o okcoin.exe okcoin.go
```

按 q键 退出程序， 按 p键 暂停 再次按 p键 继续运行

## 运行效果
![okcoin](https://github.com/yuanaichi/okcoin-terminal/blob/master/okcoin.gif?raw=true "okcoin terminal")
