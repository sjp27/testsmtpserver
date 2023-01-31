// Copyright (c) <2023> <Steve Pickford>
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

// Test SMTP server.

package main

import (
    "bufio"
    "crypto/tls"
    "encoding/base64"
    "fmt"
    "io"
    "net"
    "os"
    "strings"
)

const (
    WELCOME     string = "220 SMTP server ready\r\n"
    BYE                = "221 Bye\r\n"
    AUTHSUCCESS        = "235 Authentication successful\r\n"
    OK                 = "250 OK\r\n"
    EHLO               = "250-testemailserver\r\n250-PIPELINING\r\n250-AUTH PLAIN LOGIN\r\n250 8BITMIME\r\n"
    USERNAME           = "334 dXNlcm5hbWU6\r\n"
    PASSWORD           = "334 UGFzc3dvcmQ6\r\n"
    SEND               = "354 Send data\r\n"
    CMDNOTIMP          = "502 Command is not implemented\r\n"
    BADSEQ             = "503 Bad sequence of commands\r\n"
)

var TLSconfig *tls.Config

func main() {
    if len(os.Args) < 3 {
        fmt.Println("v1.0 Usage: testsmtpserver <port> <security (NONE, SSL)> [fail (command to stop responding e.g. AUTH)]")
        os.Exit(1)
    }

    // Get the port and fail arguments
    port := fmt.Sprintf(":%s", os.Args[1])
    security := os.Args[2]
    fail := "NONE"

    if len(os.Args) > 3 {
        fail = os.Args[3]
    }

    cert, err := tls.X509KeyPair(certPem, keyPem)
    if err != nil {
        os.Exit(1)
    }

    TLSconfig = &tls.Config{
        Certificates: []tls.Certificate{cert},
    }

    // Create tcp listener on given port
    var listener net.Listener

    if security == "NONE" {
        listener, err = net.Listen("tcp", port)
    } else if security == "SSL" {
        listener, err = tls.Listen("tcp", port, TLSconfig)
    } else {
        fmt.Println("Unsupported security type")
        os.Exit(1)
    }

    if err != nil {
        fmt.Println("Failed to create listener, err:", err)
        os.Exit(1)
    }

    fmt.Printf("Listening on port%s, fail command is %s\n", port, fail)

    // Listen for new connections
    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("Failed to accept connection, err:", err)
            continue
        }

        // Pass accepted connection to the processSMTP() goroutine
        go processSMTP(conn, fail)
    }
}

// Process SMTP commands
func processSMTP(z_conn net.Conn, z_fail string) {
    defer z_conn.Close()
    response(z_conn, WELCOME)
    reader := bufio.NewReader(z_conn)
    for {
        // Get smtp command
        cmd, err := request(reader)
        if err != nil {
            return
        }

        fmt.Printf("cmd: %s", cmd)

        command := strings.Fields(cmd)

        if len(command) >= 1 && command[0] != z_fail {
            switch command[0] {
            case "HELO":
                response(z_conn, OK)
            case "EHLO":
                response(z_conn, EHLO)
            case "AUTH":
                if command[1] == "PLAIN" {
                    if len(command) == 2 {
                        authPlain(z_conn, "")
                    } else {
                        authPlain(z_conn, command[2])
                    }
                } else if command[1] == "LOGIN" {
                    authLogin(z_conn)
                } else {
                    response(z_conn, CMDNOTIMP)
                }
            case "MAIL":
                if strings.Contains(command[1], "FROM:") {
                    response(z_conn, OK)
                }
            case "RCPT":
                if strings.Contains(command[1], "TO:") {
                    response(z_conn, OK)
                }
            case "DATA":
                data(z_conn)
            case "QUIT":
                response(z_conn, BYE)
            case "NOP":
                response(z_conn, OK)
            case "RSET":
                response(z_conn, OK)
            default:
                response(z_conn, CMDNOTIMP)
            } //switch
        } //if
    } //for
}

// Authorise user
func authPlain(z_conn net.Conn, z_userpassB64 string) {
    var userpass []byte

    if z_userpassB64 != "" {
        userpass, _ = base64.StdEncoding.DecodeString(z_userpassB64)
    } else {
        reader := bufio.NewReader(z_conn)
        response(z_conn, "354\r\n")

        // Get username/password
        userpassB64, err := request(reader)
        if err != nil {
            return
        }

        userpass, _ = base64.StdEncoding.DecodeString(userpassB64)
    }

    fmt.Printf("userpass: %s\n", userpass)

    if len(userpass) > 0 {
        response(z_conn, AUTHSUCCESS)
    } else {
        response(z_conn, BADSEQ)
    }
}

// Authorise user
func authLogin(z_conn net.Conn) {
    ok := false
    reader := bufio.NewReader(z_conn)

    response(z_conn, USERNAME)

    // Get username
    usernameB64, err := request(reader)
    if err != nil {
        return
    }

    username, _ := base64.StdEncoding.DecodeString(usernameB64)
    fmt.Printf("user: %s\n", username)

    if len(username) > 0 {
        response(z_conn, PASSWORD)

        // Get password
        passwordB64, err := request(reader)
        if err != nil {
            return
        }

        password, _ := base64.StdEncoding.DecodeString(passwordB64)
        fmt.Printf("pass: %s\n", password)

        if len(password) > 0 {
            ok = true
        }
    }

    if ok {
        response(z_conn, AUTHSUCCESS)
    } else {
        response(z_conn, BADSEQ)
    }
}

// Get email data
func data(z_conn net.Conn) {
    response(z_conn, SEND)
    reader := bufio.NewReader(z_conn)

    for {
        data, err := request(reader)
        if err != nil {
            break
        }

        fmt.Printf("data: %s", data)

        if data == ".\r\n" {
            break
        }
    }
    response(z_conn, OK)
}

// Get request
func request(z_reader *bufio.Reader) (string, error) {
    bytes, err := z_reader.ReadBytes(byte('\n'))
    if err != nil {
        if err != io.EOF {
            fmt.Println("Failed to read data, err:", err)
        }
    }
    return string(bytes), err
}

// Send response
func response(z_conn net.Conn, z_reply string) {
    z_conn.Write([]byte(z_reply))
    fmt.Printf("resp: %s\n", strings.ReplaceAll(z_reply, "\r\n", " "))
}

var certPem = []byte(`-----BEGIN CERTIFICATE-----
MIIBhzCCAS2gAwIBAgIUIEKZBvln3+8cly0/9sCRnHE5jxwwCgYIKoZIzj0EAwIw
GTEXMBUGA1UEAwwOdGVzdHNtdHBzZXJ2ZXIwHhcNMjMwMTMwMTUwMDUxWhcNMzMw
MTI3MTUwMDUxWjAZMRcwFQYDVQQDDA50ZXN0c210cHNlcnZlcjBZMBMGByqGSM49
AgEGCCqGSM49AwEHA0IABKIYQrVUv4Ef1Iz8OKjl70i0PK9DQJKr6arX/b8z8cGB
7fIDsLTFDrJOtYw2rQ5g58BHdnMJB7ghC2Y2uE/P6EyjUzBRMB0GA1UdDgQWBBQZ
tL11FqQsb/lmajeY0tsHHAN2rzAfBgNVHSMEGDAWgBQZtL11FqQsb/lmajeY0tsH
HAN2rzAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0gAMEUCICJ/v0iFIYsz
R0jRs9q7GUfcaJMGdhrUEMvsYLe43YNeAiEA5EAG9Mpc+Yi6ewSjdmByXqPRW+Tg
ftuP+r8cBo4rLtM=
-----END CERTIFICATE-----`)
var keyPem = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIFHaCYp047lMl35osaJeQRxgOPa1y/ZKHq5W51RcmpZooAoGCCqGSM49
AwEHoUQDQgAEohhCtVS/gR/UjPw4qOXvSLQ8r0NAkqvpqtf9vzPxwYHt8gOwtMUO
sk61jDatDmDnwEd2cwkHuCELZja4T8/oTA==
-----END EC PRIVATE KEY-----`)
