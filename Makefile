# Makefile for testsmtpserver
testsmtpserver: main.go
	go build -o testsmtpserver main.go
testsmtpserver.exe: main.go
	GOOS=windows GOARCH=amd64 go build -o testsmtpserver.exe main.go
clean:
	rm testsmtpserver testsmtpserver.exe