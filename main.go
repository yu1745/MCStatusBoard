package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

var (
	addr           = "0.0.0.0:25565"
	version        = "1.12.2"
	maxPlayers     = 1
	currentPlayers = 1
	description    = "@copyright https://github.com/yu1745/MCStatusBoard"
	favicon        = ""
)

func init() {
	flag.StringVar(&addr, "a", addr, "Listen address")
	flag.StringVar(&version, "v", version, "Minecraft version")
	flag.IntVar(&maxPlayers, "m", maxPlayers, "Max players")
	flag.IntVar(&currentPlayers, "c", currentPlayers, "Current players")
	flag.StringVar(&description, "d", description, "Description")
	flag.StringVar(&favicon, "f", favicon, "Favicon path")
	flag.Parse()
}

type JSONData struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
	Description struct {
		Text string `json:"text"`
	} `json:"description"`
	Favicon string `json:"favicon"`
}

var protocolMap = map[string]int{
	"1.7.2": 757,
}

func parse(s string) (net.IP, int) {
	index := strings.LastIndex(s, ":")
	if index == -1 {
		// 只有ip没有端口
		return net.ParseIP(s), 25565
	} else {
		// ip:port
		ip := net.ParseIP(s[:index])
		port, err := strconv.Atoi(s[index+1:])
		if err != nil {
			log.Fatalln(err)
		}
		return ip, port
	}
}

func main() {
	ip, port := parse(addr)
	lsAddr := &net.TCPAddr{
		IP:   ip,
		Port: port,
	}
	jsonData := JSONData{
		Version: struct {
			Name     string "json:\"name\""
			Protocol int    "json:\"protocol\""
		}{
			Name:     version,
			Protocol: protocolMap[version],
		},
		Players: struct {
			Max    int "json:\"max\""
			Online int "json:\"online\""
		}{
			Max:    maxPlayers,
			Online: currentPlayers,
		},
		Description: struct {
			Text string "json:\"text\""
		}{
			Text: description,
		},
		Favicon: favicon,
	}
	if jsonData.Version.Protocol == 0 {
		jsonData.Version.Protocol = 340 // 1.12.2
	}
	bytes, err := json.Marshal(jsonData)
	if err != nil {
		log.Fatalln(err)
	}
	str := string(bytes)
	ls, err := net.ListenTCP("tcp", lsAddr)
	if err != nil {
		log.Fatalln(err)
	}
	for {
		conn, err := ls.AcceptTCP()
		if err != nil {
			log.Fatalln(err)
		}
		go func(conn *net.TCPConn) {
			defer conn.Close()
			log.Printf("connected: %s\n", conn.RemoteAddr().String())
			defer log.Printf("disconnected: %s\n", conn.RemoteAddr().String())
			buf := make([]byte, 1024*1024)
			nR, err := conn.Read(buf)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(hex.EncodeToString(buf[:nR]))
			s := 0
			acc := 0
			//todo 检查包长度和nR,防止越界panic
			for i := 0; i < 2; i++ {
				_, acc = readVarInt(buf[s:nR])
				s += acc
			}
			protocolVersion, acc := readVarInt(buf[s:nR])
			s += acc
			log.Printf("protocolVersion:%d\n", protocolVersion)
			serverAddr, acc := readString(buf[s:nR])
			s += acc
			fmt.Printf("serverAddr: %v\n", serverAddr)
			serverPort := readUshort(buf[s:nR])
			s += 2
			fmt.Printf("serverPort: %v\n", serverPort)
			nextState, acc := readVarInt(buf[s:nR])
			s += acc
			fmt.Printf("nextState: %v\n", nextState)
			nR, err = conn.Read(buf)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println(hex.EncodeToString(buf[:nR]))
			if !(buf[0] == 1 && buf[1] == 0) {
				log.Println("error")
				return
			}
			buf = make([]byte, 1024*1024)
			nW := 0
			// nW += writeVarInt(buf[nW:], 0) // 占位
			nW += writeVarInt(buf[nW:], 0)
			nW += writeVarInt(buf[nW:], len(str))
			for i := 0; i < len(str); i++ {
				buf[nW] = str[i]
				nW++
			}
			buf2 := make([]byte, 1024*1024)
			nW2 := 0
			fmt.Printf("nW: %v\n", nW)
			nW2 += writeVarInt(buf2[nW2:], nW)
			fmt.Printf("nW2: %v\n", nW2)
			for i := 0; i < nW; i++ {
				buf2[nW2] = buf[i]
				nW2++
			}
			conn.Write(buf2[:nW2])
			println(hex.EncodeToString(buf2[:nW2]))
			// nW += writeVarInt(buf[nW:], 2)
			// nW += writeVarInt(buf[nW:], 0)
		}(conn)
	}
}

const SEGMENT_BITS = 0x7F
const CONTINUE_BIT = 0x80

// 第一个int返回data，第二个int返回varInt长度
func readVarInt(data []byte) (int, int) {

	i := 0
	var value int
	var position int
	var currentByte byte

	for {
		currentByte = data[i]
		value |= (int(currentByte) & SEGMENT_BITS) << position

		if (currentByte & CONTINUE_BIT) == 0 {
			i++
			break
		}

		position += 7

		if position >= 32 {
			panic("VarInt is too big")
		}
		i++
	}

	return value, i
}

func writeVarInt(data []byte, value int) int {
	var i int
	for {
		data[i] = byte(value & SEGMENT_BITS)
		value >>= 7
		if value == 0 {
			i++
			break
		}
		data[i] |= CONTINUE_BIT
		i++
	}
	return i
}

func readString(data []byte) (string, int) {
	l, acc := readVarInt(data)
	return string(data[acc : acc+l]), acc + l
}

func readUshort(data []byte) uint16 {
	return uint16(data[0])<<8 | uint16(data[1])
}
