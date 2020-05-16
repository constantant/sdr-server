package main

import (
	"encoding/binary"
	"github.com/googollee/go-socket.io"
	"github.com/mjibson/go-dsp/fft"
	endian "github.com/virtao/GoEndian"
	"log"
	"net"
	"net/http"
	"sync"
)

var r1c1cbs = make(map[string]func(data []float64), 1)
var r1c2cbs = make(map[string]func(data []float64), 1)

var wg sync.WaitGroup

func main() {
	wg.Add(2)
	go socketStream()
	go udpStream()
	wg.Wait()

}

func socketStream() {
	server, err := socketio.NewServer(nil)
	if err != nil {
		log.Fatal(err)
	}
	server.On("connection", func(so socketio.Socket) {
		log.Println("on connection")
		so.Join("r1c1")
		so.Join("r1c2")
		r1c1cb := func(data []float64) {
			// log.Println("r1c1", data)
			so.Emit("r1c1", data)
		}
		r1c1cbs[so.Id()] = r1c1cb
		r1c2cb := func(data []float64) {
			// log.Println("r1c2", data)
			so.Emit("r1c2", data)
		}
		r1c2cbs[so.Id()] = r1c2cb

		so.On("r1c1", func(data []float64) {
			so.BroadcastTo("r1c1", "r1c1", data)
		})
		so.On("r1c2", func(data []float64) {
			so.BroadcastTo("r1c2", "r1c2", data)
		})
		so.On("disconnection", func(so socketio.Socket) {
			delete(r1c1cbs, so.Id())
			delete(r1c2cbs, so.Id())
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

func udpStream() {
	// listen to incoming udp packets
	pc, err := net.ListenPacket("udp", ":12345")
	if err != nil {
		log.Fatal(err)
	}
	defer pc.Close()

	//Connect udp
	conn2, err := net.Dial("udp", ":12346")
	if err != nil {
		return
	}
	defer conn2.Close()

	handlePackets(&pc, &conn2)

	wg.Done()
}

func handlePackets(pc *net.PacketConn, conn2 *net.Conn) {
	mid := 8388608
	scale24 := 1.0 / 8388607.0
	var _lut3D24bit [256][256][256]float64

	for i := 0; i < 16777216; i++ {
		x := i - mid
		pb := make([]byte, 4)
		val := float64(x) * scale24
		if endian.IsLittleEndian() {
			binary.LittleEndian.PutUint32(pb, uint32(x))
			_lut3D24bit[pb[0]][pb[1]][pb[2]] = val
		} else {
			binary.BigEndian.PutUint32(pb, uint32(x))
			_lut3D24bit[pb[3]][pb[2]][pb[1]] = val
		}
	}

	_bufferSize := 21600

	for {
		// log.Print("i")
		//simple read
		recPtr := make([]byte, _bufferSize)
		bytesRec, _, err := (*pc).ReadFrom(recPtr)
		if err != nil {
			log.Fatal("ReadFrom", err)
			return
		}

		//test
		(*conn2).Write(recPtr)
		// log.Print("i", bytesRec)
		go calcLuts(recPtr, bytesRec, &_lut3D24bit)
		// time.Sleep(time.Second)
	}
}

func calcLuts(recPtr []byte, bytesRec int, lut3D24bit *[256][256][256]float64) {
	_lut3D24bit := *lut3D24bit
	ptr1 := make([]complex128, 0)
	ptr2 := make([]complex128, 0)
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
	sendData(ptr1, ptr2)
}

func sendData(ptr1 []complex128, ptr2 []complex128) {
	fft1 := fft.FFT(ptr1)
	fft2 := fft.FFT(ptr2)

	var dataR1C1 []float64
	var dataR1C2 []float64

	for _, v := range fft1 {
		dataR1C1 = append(dataR1C1, real(v))
		dataR1C1 = append(dataR1C1, imag(v))
	}
	for _, v := range fft2 {
		dataR1C2 = append(dataR1C2, real(v))
		dataR1C2 = append(dataR1C2, imag(v))
	}

	for _, cb := range r1c1cbs {
		cb(dataR1C1)
	}
	for _, cb := range r1c2cbs {
		cb(dataR1C2)
	}

	// sentDataX = sentDataX + 1
	// log.Println("Data is sent", sentDataX)
}
