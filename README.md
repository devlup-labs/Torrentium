# 🔥 Torrentium

**Direct peer-to-peer file sharing that works through firewalls!**

Torrentium is a WebRTC-based P2P file sharing application that allows direct file transfer between two computers without requiring port forwarding, VPN setup, or cloud services.

## ✨ Features

- 🚀 **Direct P2P Connection** - Files transfer directly between computers
- 🔒 **No Port Forwarding** - Works through NAT and firewalls automatically
- 🔐 **Encrypted Transfers** - All data is encrypted by WebRTC
- 🌍 **Works Anywhere** - Mobile data, corporate networks, home WiFi
- 📱 **Simple Setup** - Just copy-paste two JSON strings
- ⚡ **Fast Transfer** - Direct connection, no intermediary servers

## 🚀 Quick Start

### 1. Build and Run
```bash
go build
./torrentium
```

### 2. Connection Setup
**Person A (Initiator):**
```
> offer
[Copy the generated offer JSON]
```

**Person B (Responder):**
```
> answer <paste_offer_json_here>
[Copy the generated answer JSON]
```

**Person A (Complete):**
```
> complete <paste_answer_json_here>
✅ Connection established!
```

### 3. Transfer Files
Either person can now request files:
```
> download filename.txt
```

## 📋 Commands

- `offer` - Create connection offer (start here)
- `answer <offer>` - Answer connection offer
- `complete <answer>` - Complete connection with answer
- `download <file>` - Download file from peer
- `status` - Show connection status
- `help` - Show instructions
- `exit` - Quit application

## 🔧 Requirements

- Go 1.21 or later
- Internet connection (for initial WebRTC signaling)
- Files to share in the same directory

## 🌐 How It Works

Torrentium uses WebRTC technology to establish direct peer-to-peer connections:

1. **Signaling Phase**: Peers exchange connection information (offer/answer)
2. **NAT Traversal**: WebRTC automatically handles firewall/NAT issues using STUN servers
3. **Direct Connection**: Once established, files transfer directly between computers
4. **Encrypted Transfer**: All data is automatically encrypted by WebRTC

## 🛠️ Building from Source

```bash
git clone <repository-url>
cd torrentium
go mod tidy
go build
```

## 📝 Notes

- Downloaded files are saved with `downloaded_` prefix
- Both computers need internet access for initial connection setup
- After connection is established, transfer works even on local networks
- Works on Windows, Linux, and macOS

## 🤝 Contributing

Feel free to contribute improvements, bug fixes, or new features!

## 📦 Dependencies and Imports

- go mod init github.com/1amKhush/Practice-
- go get github.com/pion/webrtc/v4
- go get github.com/pion/ice/v2
- go get github.com/libp2p/go-libp2p
- go get github.com/libp2p/go-libp2p-pubsub
- go get github.com/libp2p/go-libp2p/p2p/discovery/mdns
- go get github.com/libp2p/go-libp2p-kad-dht
- go get github.com/multiformats/go-multiaddr
- go get github.com/joho/godotenv
- go get github.com/lib/pq


- go mod tidy


---

**Made with ❤️ for seamless P2P file sharing**
