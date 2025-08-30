package torrentfile

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"

	bencode "github.com/jackpal/bencode-go"
)

//yeh struct .torrentl file ka metadata define karta hai.Bencode format mein encode hota hai.
type TorrentMeta struct {
	Filename  string    `bencode:"filename"`
	Length    int64     `bencode:"length"`
	Hash      string    `bencode:"hash"`
	CreatedAt int64 `bencode:"created_at"`
}


// CreateTorrentFile function di gayi file ke liye ek .torrent file banata hai.
// Yeh file ka metadata (naam, size, hash) collect karta hai aur use bencode format mein save karta hai.
func CreateTorrentFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	//file ka data fetch kara hai yha
	info, err := file.Stat()
	if err != nil {
		return err
	}

	hashCalc := sha256.New()
	if _, err = io.Copy(hashCalc, file); err != nil {
		return err
	}
	//hexadecimal string mein convert kardiya hash ko
	hexHash := hex.EncodeToString(hashCalc.Sum(nil))

	meta := TorrentMeta{
		Filename:  info.Name(),
		Length:    info.Size(),
		Hash:      hexHash,
		CreatedAt: time.Now().Unix(),
	}

	outputName := filename + ".torrent"
	out, err := os.Create(outputName)
	if err != nil {
		return err
	}
	defer out.Close()

	// Metadata struct ko bencode format mein encode karke output file mein likhte hain.
	return bencode.Marshal(out, meta)
}