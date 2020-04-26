package main

import (
	"encoding/binary"
	"github.com/googollee/go-socket.io"
	endian "github.com/virtao/GoEndian"
	"log"
	"net"
	"net/http"
	"sync"
)

type Subscriber *func(data []byte)

var fn = make(chan Subscriber, 1)

var wg sync.WaitGroup

func main() {
	wg.Add(3)
	go udpStream()
	go tcpStream()
	go socketStream()
	wg.Wait()

}

func socketStream() {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.On("connection", func(so socketio.Socket) {
		log.Println("on connection")
		so.Join("data")
		f1 := func(data []byte) {
			var values []int
			for _, v := range data {
				values = append(values, int(v))
			}
			log.Println("data", data)
			log.Println("emit:", so.Emit("data message", values))
		}
		fn <- &f1
		so.On("data message", func(data []byte) {
			so.BroadcastTo("data", "data message", data)
		})
		so.On("disconnection", func() {
			log.Println("on disconnect")
		})
	})
	server.On("error", func(so socketio.Socket, err error) {
		log.Println("error:", err)
	})

	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./asset")))
	log.Println("Serving at localhost:5000...")
	log.Fatal(http.ListenAndServe(":5000", nil))
	wg.Done()
}

func tcpStream() {
	p1 := <-fn
	// listen to incoming tcp connections
	l, err := net.Listen("tcp", ":6340")
	if err != nil {
		return
	}
	defer l.Close()
	// A common pattern is to start a loop to continously accept connections
	for {
		//accept connections using Listener.Accept()
		c, err := l.Accept()
		if err != nil {
			return
		}
		//It's common to handle accepted connection on different goroutines
		go handleConnection(c, p1)
	}
	wg.Done()
}

func handleConnection(c net.Conn, p Subscriber) {
	b := make([]byte, 4)
	c.Read(b)
	(*p)(b)
}

func udpStream() {
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", ":12345")
	if err != nil {
		log.Fatal(err)
	}
	defer pc.Close()

	handlePackets(&pc)

	wg.Done()
}

func handlePackets(pc *net.PacketConn) {
	mid := 8388608
	scale24 := float32(1.0 / 8388607.0)
	var _lut3D24bit [256][256][256]float32

	for i := 0; i < 16777216; i++ {
		x := i - mid
		pb := make([]byte, 4)
		binary.LittleEndian.PutUint32(pb, uint32(x))
		val := float32(x) * scale24
		if endian.IsLittleEndian() {
			_lut3D24bit[pb[0]][pb[1]][pb[2]] = val
		} else {
			_lut3D24bit[pb[3]][pb[2]][pb[1]] = val
		}
	}

	_bufferSize := 36

	for {
		ptr1 := make([]complex64, 0)
		ptr2 := make([]complex64, 0)
		//simple read
		recPtr := make([]byte, _bufferSize)
		(*pc).ReadFrom(recPtr)

		bytesRec := len(recPtr)
		iterations := bytesRec / 12

		for i := 0; i < iterations; i++ {
			Ix := i * 12
			ptr1Value := complex(
				_lut3D24bit[recPtr[Ix+5]][recPtr[Ix+4]][recPtr[Ix+3]],
				_lut3D24bit[recPtr[Ix+2]][recPtr[Ix+1]][recPtr[Ix]])
			ptr2Value := complex(
				_lut3D24bit[recPtr[Ix+11]][recPtr[Ix+10]][recPtr[Ix+9]],
				_lut3D24bit[recPtr[Ix+8]][recPtr[Ix+7]][recPtr[Ix+6]])
			ptr1 = append(ptr1, ptr1Value)
			ptr2 = append(ptr2, ptr2Value)
		}
		log.Println("udp stream", ptr1, ptr2)
	}
}
