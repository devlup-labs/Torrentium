export const mockUser = {
  username: "TechNinja42",
  peerId: "TR-A8F3E2D9C1B6H5K7",
  avatar: null,
  trustScore: 87,
  totalShared: 156,
  totalDownloaded: 89,
  badges: ["Early Adopter", "Speed Demon", "Trusted Seeder", "Community Helper", "File Wizard"]
}

export const mockTransfers = [
  {
    id: "1",
    name: "Ubuntu 22.04.3 Desktop.iso",
    size: 4698415104,
    type: "ISO Image",
    status: "completed",
    progress: 100,
    speed: 0,
    date: "2024-01-15T10:30:00Z",
    hash: "a8b3c4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7"
  },
  {
    id: "2", 
    name: "VS Code Extensions Pack.zip",
    size: 524288000,
    type: "Archive",
    status: "downloading",
    progress: 67,
    speed: 15728640,
    date: "2024-01-16T14:22:00Z",
    hash: "b9c4d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7a8"
  },
  {
    id: "3",
    name: "React Documentation.pdf",
    size: 8388608,
    type: "Document",
    status: "uploading",
    progress: 34,
    speed: 2097152,
    date: "2024-01-16T16:45:00Z",
    hash: "c0d5e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7a8b9"
  },
  {
    id: "4",
    name: "Design Assets Collection.zip",
    size: 1073741824,
    type: "Archive", 
    status: "failed",
    progress: 23,
    speed: 0,
    date: "2024-01-16T12:15:00Z",
    hash: "d1e6f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7a8b9c0"
  },
  {
    id: "5",
    name: "Node.js v20 LTS.msi",
    size: 33554432,
    type: "Installer",
    status: "completed",
    progress: 100,
    speed: 0,
    date: "2024-01-14T09:12:00Z",
    hash: "e2f7g8h9i0j1k2l3m4n5o6p7q8r9s0t1u2v3w4x5y6z7a8b9c0d1"
  }
]

export const mockPeers = [
  {
    id: "peer1",
    username: "CodeMaster88",
    avatar: null,
    trustScore: 92,
    lastSeen: "2024-01-16T17:30:00Z",
    isOnline: true,
    isBlocked: false
  },
  {
    id: "peer2", 
    username: "DataHoarder",
    avatar: null,
    trustScore: 76,
    lastSeen: "2024-01-16T16:45:00Z",
    isOnline: true,
    isBlocked: false
  },
  {
    id: "peer3",
    username: "SpeedSeeder",
    avatar: null,
    trustScore: 95,
    lastSeen: "2024-01-16T15:20:00Z",
    isOnline: false,
    isBlocked: false
  },
  {
    id: "peer4",
    username: "TorrentKing",
    avatar: null,
    trustScore: 68,
    lastSeen: "2024-01-16T14:10:00Z",
    isOnline: true,
    isBlocked: false
  },
  {
    id: "peer5",
    username: "FileSharingPro",
    avatar: null,
    trustScore: 84,
    lastSeen: "2024-01-15T22:30:00Z",
    isOnline: false,
    isBlocked: true
  }
]

export const mockLeaderboard = [
  {
    id: "lead1",
    username: "UltraSeeder",
    avatar: null,
    badgeCount: 15,
    trustScore: 98,
    totalShared: 500,
    rank: 1
  },
  {
    id: "lead2",
    username: "SpeedDemon",
    avatar: null,
    badgeCount: 12,
    trustScore: 95,
    totalShared: 387,
    rank: 2
  },
  {
    id: "lead3",
    username: "DataVault",
    avatar: null,
    badgeCount: 11,
    trustScore: 94,
    totalShared: 356,
    rank: 3
  },
  {
    id: "lead4",
    username: "ShareMaster",
    avatar: null,
    badgeCount: 10,
    trustScore: 92,
    totalShared: 289,
    rank: 4
  },
  {
    id: "lead5",
    username: "TechNinja42",
    avatar: null,
    badgeCount: 5,
    trustScore: 87,
    totalShared: 156,
    rank: 5
  }
]

export const badgesList = [
  "Early Adopter",
  "Speed Demon", 
  "Trusted Seeder",
  "Community Helper",
  "File Wizard",
  "Data Guardian",
  "Network Pioneer",
  "Upload Champion",
  "Download Master",
  "Peer Connector",
  "Torrent Expert",
  "Sharing Enthusiast",
  "Tech Savvy",
  "Reliability King",
  "Bandwidth Hero"
]
