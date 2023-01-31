# Makefile for testsmtpserver
testsmtpserver: main.go
	GOOS=linux GOARCH=amd64 go build -o testsmtpserver main.go
testsmtpserver.exe: main.go
	GOOS=windows GOARCH=amd64 go build -o testsmtpserver.exe main.go
release:
	zip testsmtpserver.zip testsmtpserver testsmtpserver.exe		
clean:
	rm testsmtpserver testsmtpserver.exe
