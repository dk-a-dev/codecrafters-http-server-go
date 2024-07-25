package main
import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)
var statusCodeToString = map[int]string{
	200: "OK",
	404: "Not Found",
}
type Request struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

func (r Response) String() string {
	statusText, ok := statusCodeToString[r.StatusCode]
	if !ok {
		statusText = "Unknown"
	}
	
	if r.Headers == nil {
		r.Headers = map[string]string{
			"Content-Type": "text/plain",
		}
	}

	if _, ok = r.Headers["Content-Length"]; !ok {
		r.Headers["Content-Length"] = strconv.Itoa(len(r.Body))
	}

	var headerString strings.Builder
	for k, v := range r.Headers {
		headerString.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	return fmt.Sprintf("HTTP/1.1 %d %s\r\n%s\r\n%s", r.StatusCode, statusText, headerString.String(), r.Body)
}

func main() {
	l, err := net.Listen("tcp", ":4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	stream := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println("Failed to close connection: ", err.Error())
		}
	}(conn)

	request, err := parseRequest(stream.Reader)
	if err != nil {
		fmt.Println("Failed to parse request: ", err.Error())
		os.Exit(1)
	}

	response := Response{StatusCode: 200}
	if strings.HasPrefix(request.Path, "/echo") {
		pathParts := strings.SplitN(request.Path, "/echo/", 2)
		response.Body = pathParts[1]
	} else if request.Path == "/user-agent" {
		userAgent := request.Headers["User-Agent"]
		response.Body = userAgent
	} else if request.Path != "/" {
		response.StatusCode = 404
	}
	_, err = stream.WriteString(response.String())
	if err != nil {
		fmt.Println("Failed to write to socket: ", err.Error())
		os.Exit(1)
	}

	err = stream.Flush()
	if err != nil {
		fmt.Println("Failed to flush to socket")
		os.Exit(1)
	}
}

func parseRequest(reader *bufio.Reader) (Request, error) {
	request := Request{
		Headers: make(map[string]string),
	}
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return Request{}, fmt.Errorf("malformed HTTP request")
	}
	parts := strings.Split(firstLine, " ")
	request.Method = parts[0]
	request.Path = parts[1]
	for {
		curLine, err := reader.ReadString('\n')
		if curLine == "\r\n" {
			break
		}
		if err == io.EOF {
			return request, nil
		} else if err != nil {
			return Request{}, err
		}
		headerParts := strings.SplitN(curLine, ":", 2)
		request.Headers[headerParts[0]] = strings.TrimSpace(headerParts[1])
	}

	contentLenStr, ok := request.Headers["Content-Length"]
	if !ok {
		return request, nil
	}
	contentLen, _ := strconv.Atoi(contentLenStr)
	buf := make([]byte, contentLen)

	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return request, err
	}
	request.Body = string(buf)
	return request, nil
}