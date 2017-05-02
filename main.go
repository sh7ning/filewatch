package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "net/http"
	"time"
    "html/template"

	"github.com/gorilla/websocket"
    "github.com/hpcloud/tail"
)

const (
	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var (
    upgrader  = websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
    }
)

func reader(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writer(ws *websocket.Conn, filename string) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		ws.Close()
	}()

    firstWrite(ws)

    //默认不输出数据 只读新追的数据
    var n = int64(0)
    t, _ := tail.TailFile(filename, tail.Config{
        // ReOpen:   true,
        Poll:     true,
        Follow:   true,
        Location: &tail.SeekInfo{-n, os.SEEK_END},
    })

	for {
		select {
        case line, _ := <-t.Lines:
			if line != nil {
				ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.TextMessage, []byte(line.Text)); err != nil {
					return
				}
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

//TODO 首次输出文件最后的几行
func firstWrite(ws *websocket.Conn) {
}

func getFilename(r *http.Request) string {
    // TODO 基础目录配置
    baseDir := map[string]string{
        "filewatch": "/Users/develop/workspace/study/golang/src/filewatch/",
    }
    // TODO filename参数 为空的处理，同时为了安全:filename 拼接后必须在 配置目录之下，不允许执行到其他目录
    return baseDir["filewatch"] + getRawFilename(r)
}

func getRawFilename(r *http.Request) string {
    filename := r.FormValue("filename");
    // TODO 404 文件名为空的时候异常处理
    // if filename == "" {
    //     log.Println("文件名为空")
    //
    //     return nil
    // }

    return filename
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
    filename := getFilename(r)

	go writer(ws, filename)
	reader(ws)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

    // 加载index.html页面
    // http.ServeFile(w, r, view_file["index"])
    // fmt.Fprint(w, homeHTML)
    filename := getRawFilename(r)
	var v = struct {
		Filename string
	}{
		filename,
	}

    homeTempl := template.Must(template.New("").Parse(homeHTML))
	homeTempl.Execute(w, &v)
}

func main() {
	var addr = flag.String("addr", ":8080", "http service address")
	flag.Parse()
	// if flag.NArg() == 0 {
	// 	log.Fatal("filename not specified")
	// }
	// filename = flag.Args()[0]
    fmt.Println("file watch with port " + *addr)

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

const homeHTML = `<!DOCTYPE html>
<html lang="en">
    <head>
        <title>file watcher</title>
    </head>
    <body>
        <h3>File Watcher</h3>
        <h5>filename: {{.Filename}}</h5>
        <p id="log">
        </p>
        <script type="text/javascript">
        function appendLog(data) {
            var item = document.createElement("div");
            item.innerText = data;

            var logElem = document.getElementById("log")
            logElem.appendChild(item);

            var doScroll = logElem.scrollTop > logElem.scrollHeight - logElem.clientHeight - 1;
            if (doScroll) {
                logElem.scrollTop = logElem.scrollHeight - logElem.clientHeight;
            }
        }

        (function() {
            var conn = new WebSocket("ws://" + document.location.host + "/ws?filename={{.Filename}}");
            conn.onclose = function(evt) {
                appendLog('Connection closed');
            }
            conn.onmessage = function(evt) {
                appendLog(evt.data);
            }
        })();
        </script>
    </body>
</html>
`
