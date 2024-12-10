package util

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"sort"
	"strings"
	"time"
)

func getIpFromAddr(addr net.Addr) net.IP {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	}
	if ip == nil || ip.IsLoopback() {
		return nil
	}
	ip = ip.To4()
	if ip == nil {
		return nil // not an ipv4 address
	}
	return ip
}

// ExternalIP
func _() (net.IP, error) {
	iFaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iFace := range iFaces {
		if iFace.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iFace.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		address, e := iFace.Addrs()
		if e != nil {
			return nil, e
		}
		for _, addr := range address {
			ip := getIpFromAddr(addr)
			if ip == nil {
				continue
			}
			return ip, nil
		}
	}
	return nil, errors.New("not connected to the network")
}

func ComposeParams(body map[string]interface{}) (params string) {
	keys := make([]string, 0, len(body))
	var buf strings.Builder
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if buf.Len() > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(key)
		buf.WriteByte('=')
		buf.WriteString(fmt.Sprintf("%v", body[key]))
		//buf.WriteString(body[key].(string))
	}
	return buf.String()
}

////	method: GET, POST, DELETE
//func HttpRequestBybit(method string, reqUrl string, body string, requestHeaders map[string]string, form url.Values) (
//	[]byte, error) {
//	req, _ := http.NewRequest(method, reqUrl, strings.NewReader(body))
//	buf := &bytes.Buffer{}
//	w := multipart.NewWriter(buf)
//	for k, v := range form {
//		for _, iv := range v {
//			_ := w.WriteField(k, iv)
//		}
//	}
//	_ = w.Close()
//	if requestHeaders != nil {
//		for k, v := range requestHeaders {
//			req.Header.Add(k, v)
//		}
//	}
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		SocketInfo("can not process request " + err.Error())
//		return nil, err
//	}
//	defer resp.Body.Close()
//
//	bodyData, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		SocketInfo("can not read message from request " + err.Error())
//		return nil, err
//	}
//	if resp.StatusCode != 200 {
//		SocketInfo(fmt.Sprintf("%sHttpStatusCode:%d ,Desc:%s", reqUrl, resp.StatusCode, string(bodyData)))
//	}
//	return bodyData, nil
//}

// HttpRequest method: GET, POST, DELETE
func HttpRequest(method string, reqUrl string, body string, requestHeaders map[string]string, timeout int) ([]byte, error) {
	req, createErr := http.NewRequest(method, reqUrl, strings.NewReader(body))
	if createErr != nil {
		Log(LogLevelError, `fail to request http`+createErr.Error())
		return nil, createErr
	}
	if requestHeaders != nil {
		for k, v := range requestHeaders {
			req.Header.Add(k, v)
		}
	}
	req.Header.Add(`connection`, `Keep-Alive`)
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*time.Duration(timeout))
	defer cncl()
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		Log(LogLevelError, "can not process request "+err.Error())
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			Log(LogLevelError, `fail to request, return `+err.Error())
		}
	}(resp.Body)
	bodyData, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		Log(LogLevelError, "can not read message from request "+readErr.Error())
		return nil, err
	}
	if resp.StatusCode != 200 {
		Log(LogLevelError, fmt.Sprintf("%sHttpStatusCode:%d ,Desc:%s", reqUrl, resp.StatusCode, string(bodyData)))
	}
	return bodyData, nil
}

func SendMail(fromAddress, mailAuth, toAddress, subject, body string) (err error) {
	//Notice(fmt.Sprintf(`pretend to send %s %s %s %s %s`, fromAddress, mailAuth, toAddress, subject, body))
	//return nil
	from := mail.Address{Address: fromAddress}
	to := mail.Address{Address: toAddress}
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = subject
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body
	servername := "smtp.qq.com:465"
	host, _, _ := net.SplitHostPort(servername)
	auth := smtp.PlainAuth("", fromAddress, mailAuth, host)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	conn, err := tls.Dial("tcp", servername, tlsConfig)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	// Auth
	if err = c.Auth(auth); err != nil {
		return err
	}
	// To && From
	if err = c.Mail(from.Address); err != nil {
		return err
	}
	if err = c.Rcpt(to.Address); err != nil {
		return err
	}
	// Data
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(message))
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	_ = c.Quit()
	Log(LogLevelError, fmt.Sprintf(`%s to %s %s %s`, from.String(), to.String(), subject, message))
	return err
}
