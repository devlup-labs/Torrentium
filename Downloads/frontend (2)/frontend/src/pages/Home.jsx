import React, { useState, useRef } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { 
  Upload, 
  Download, 
  Users, 
  TrendingUp,
  File,
  Plus,
  Search,
  FileUp,
  FileDown
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Progress } from '../components/ui/Progress'
import { Badge } from '../components/ui/Badge'
import { formatBytes, formatSpeed, formatDate } from '../lib/utils'

export function Home() {
  const { user, transfers, peers, addNotification } = useApp()
  const navigate = useNavigate()
  const fileInputRef = useRef(null)
  const torrentInputRef = useRef(null)
  const [dragOver, setDragOver] = useState(false)
  const [torrentDragOver, setTorrentDragOver] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const uploadTransfers = transfers.filter(t => 
    t.status === 'uploading' || (t.status === 'completed' && t.type === 'upload')
  )
  
  const downloadTransfers = transfers.filter(t => 
    t.status === 'downloading' || (t.status === 'completed' && t.type === 'download')
  )

  const onlinePeers = peers.filter(p => p.isOnline && !p.isBlocked).length

  // Mock trending files data
  const trendingFiles = [
    {
      id: 'trend1',
      name: 'Ubuntu 22.04.4 LTS Desktop.iso',
      size: 4698415104,
      type: 'OS',
      transfers: 1247,
      seeders: 89,
      leechers: 23
    },
    {
      id: 'trend2',
      name: 'Complete Web Development Course 2024.zip',
      size: 8589934592,
      type: 'Education',
      transfers: 892,
      seeders: 156,
      leechers: 45
    },
    {
      id: 'trend3',
      name: 'Blender 4.0 LTS.dmg',
      size: 2147483648,
      type: 'Software',
      transfers: 734,
      seeders: 67,
      leechers: 12
    },
    {
      id: 'trend4',
      name: 'Programming Textbooks Collection.rar',
      size: 1073741824,
      type: 'Books',
      transfers: 623,
      seeders: 98,
      leechers: 34
    },
    {
      id: 'trend5',
      name: 'Adobe Creative Suite 2024.zip',
      size: 12884901888,
      type: 'Software',
      transfers: 567,
      seeders: 43,
      leechers: 67
    }
  ]

  const filteredTrendingFiles = trendingFiles.filter(file =>
    file.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    file.type.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const stats = [
    {
      title: "Files Shared",
      value: user.totalShared,
      icon: Upload,
      description: "Total files uploaded",
      color: "text-green-500"
    },
    {
      title: "Files Downloaded", 
      value: user.totalDownloaded,
      icon: Download,
      description: "Total files downloaded",
      color: "text-blue-500"
    },
    {
      title: "Trust Score",
      value: user.trustScore,
      icon: TrendingUp,
      description: "Community reputation",
      color: "text-yellow-500"
    },
    {
      title: "Online Peers",
      value: onlinePeers,
      icon: Users,
      description: "Connected peers",
      color: "text-green-500"
    }
  ]

  const handleFileUpload = (files) => {
    Array.from(files).forEach(file => {
      addNotification(`Upload Started: Started uploading ${file.name}`, 'success')
    })
    navigate('/upload')
  }

  const handleTorrentUpload = (files) => {
    Array.from(files).forEach(file => {
      if (file.name.endsWith('.torrent')) {
        addNotification(`Torrent Added: Added ${file.name} to downloads`, 'success')
      } else {
        addNotification(`Invalid File: Please upload a .torrent file`, 'error')
      }
    })
    navigate('/download')
  }

  const handleDrop = (e) => {
    e.preventDefault()
    setDragOver(false)
    const files = e.dataTransfer.files
    if (files.length > 0) {
      handleFileUpload(files)
    }
  }

  const handleTorrentDrop = (e) => {
    e.preventDefault()
    setTorrentDragOver(false)
    const files = e.dataTransfer.files
    if (files.length > 0) {
      handleTorrentUpload(files)
    }
  }

  const handleDragOver = (e) => {
    e.preventDefault()
    setDragOver(true)
  }

  const handleTorrentDragOver = (e) => {
    e.preventDefault()
    setTorrentDragOver(true)
  }

  const handleDragLeave = (e) => {
    e.preventDefault()
    setDragOver(false)
  }

  const handleTorrentDragLeave = (e) => {
    e.preventDefault()
    setTorrentDragOver(false)
  }

  const handleDownloadTrending = (file) => {
    addNotification(`Download Started: Started downloading ${file.name}`, 'success')
    navigate('/download')
  }

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return 'success'
      case 'downloading': return 'default'
      case 'uploading': return 'secondary'
      case 'failed': return 'destructive'
      default: return 'outline'
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Welcome back, {user.username}!</h1>
        <p className="text-text-secondary mt-2">
          Here's what's happening with your torrents
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {stats.map((stat) => (
          <Card key={stat.title}>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                {stat.title}
              </CardTitle>
              <stat.icon className={`h-4 w-4 ${stat.color}`} />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{stat.value}</div>
              <p className="text-xs text-text-secondary">
                {stat.description}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Search Bar */}
      <Card>
        <CardHeader className="pb-4">
          <CardTitle className="flex items-center space-x-2">
            <Search className="h-5 w-5" />
            <span>Search Files</span>
          </CardTitle>
          <CardDescription>
            Find and download popular files from the network
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-text-secondary" />
            <Input
              placeholder="Search for files, software, movies, music..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>
        </CardContent>
      </Card>

      {/* Upload and Download Sections */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Upload Files Section */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center space-x-2">
              <FileUp className="h-5 w-5 text-green-500" />
              <span>Upload Files</span>
            </CardTitle>
            <CardDescription>
              Share your files with the network
            </CardDescription>
          </CardHeader>
          <CardContent>
            {/* Drag & Drop Zone */}
            <div
              className={`border-2 border-dashed rounded-lg p-6 text-center transition-colors ${
                dragOver 
                  ? 'border-primary bg-primary/10' 
                  : 'border-background hover:border-primary/50'
              }`}
              onDrop={handleDrop}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
            >
              <Upload className="h-10 w-10 mx-auto mb-3 text-text-secondary" />
              <p className="font-medium mb-2">Drop files here to upload</p>
              <p className="text-sm text-text-secondary mb-3">
                or click to browse files
              </p>
              <Button 
                onClick={() => fileInputRef.current?.click()}
                className="mb-3"
              >
                <Plus className="h-4 w-4 mr-2" />
                Select Files
              </Button>
              <input
                type="file"
                ref={fileInputRef}
                multiple
                className="hidden"
                onChange={(e) => handleFileUpload(e.target.files)}
              />
              <div className="flex justify-center space-x-2 text-xs text-text-secondary">
                <Button variant="outline" size="sm" asChild>
                  <Link to="/upload">Advanced Upload</Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Download Files Section */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center space-x-2">
              <FileDown className="h-5 w-5 text-blue-500" />
              <span>Download Files</span>
            </CardTitle>
            <CardDescription>
              Download files from the network
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">

            {/* Upload Torrent File */}
            <div
              className={`border-2 border-dashed rounded-lg p-4 text-center transition-colors ${
                torrentDragOver 
                  ? 'border-primary bg-primary/10' 
                  : 'border-background hover:border-primary/50'
              }`}
              onDrop={handleTorrentDrop}
              onDragOver={handleTorrentDragOver}
              onDragLeave={handleTorrentDragLeave}
            >
              <File className="h-6 w-6 mx-auto mb-2 text-text-secondary" />
              <p className="font-medium mb-1">Upload .torrent file</p>
              <p className="text-sm text-text-secondary mb-2">
                Drop torrent file or click to browse
              </p>
              <Button 
                onClick={() => torrentInputRef.current?.click()}
                variant="outline"
                size="sm"
              >
                <Plus className="h-3 w-3 mr-2" />
                Select Torrent
              </Button>
              <input
                type="file"
                ref={torrentInputRef}
                accept=".torrent"
                className="hidden"
                onChange={(e) => handleTorrentUpload(e.target.files)}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Trending Files */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <TrendingUp className="h-5 w-5 text-yellow-500" />
            <span>Trending Files</span>
          </CardTitle>
          <CardDescription>
            Most popular files with high transfer activity
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {filteredTrendingFiles.map((file, index) => (
              <div key={file.id} className="flex items-center justify-between p-3 rounded-lg bg-background/40 hover:bg-background/60 transition-colors">
                <div className="flex items-center space-x-4">
                  <div className="flex items-center justify-center w-8 h-8 rounded-full bg-primary/20 text-primary font-bold text-sm">
                    {index + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="font-medium truncate">{file.name}</p>
                    <div className="flex items-center space-x-3 text-sm text-text-secondary">
                      <Badge variant="outline" className="text-xs">
                        {file.type}
                      </Badge>
                      <span>{formatBytes(file.size)}</span>
                      <span>{file.transfers} transfers</span>
                      <span className="text-green-500">{file.seeders} seeders</span>
                      <span className="text-red-500">{file.leechers} leechers</span>
                    </div>
                  </div>
                </div>
                <Button 
                  onClick={() => handleDownloadTrending(file)}
                  size="sm"
                  className="ml-4"
                >
                  <Download className="h-4 w-4 mr-2" />
                  Download
                </Button>
              </div>
            ))}
          </div>
          
          {filteredTrendingFiles.length === 0 && searchQuery && (
            <div className="text-center py-8 text-text-secondary">
              <Search className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No files found matching "{searchQuery}"</p>
              <p className="text-sm">Try a different search term</p>
            </div>
          )}
          
          <div className="mt-6 text-center">
            <Button variant="outline" asChild>
              <Link to="/download">
                View All Files
              </Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
