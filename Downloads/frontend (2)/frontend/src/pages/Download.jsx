import React, { useState, useRef, useCallback } from 'react'
import { 
  Download as DownloadIcon, 
  FolderOpen, 
  Play, 
  Pause, 
  Users,
  Activity,
  Hash,
  AlertCircle
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Progress } from '../components/ui/Progress'
import { Badge } from '../components/ui/Badge'
import { formatBytes, formatSpeed } from '../lib/utils'

export function Download() {
  const { transfers, dispatch, addNotification } = useApp()
  const [downloadLocation, setDownloadLocation] = useState('C:\\Downloads\\Torrentium')
  const [isDragOver, setIsDragOver] = useState(false)
  const fileInputRef = useRef(null)

  const activeDownloads = transfers.filter(t => t.status === 'downloading')

  const handleDragOver = useCallback((e) => {
    e.preventDefault()
    setIsDragOver(true)
  }, [])

  const handleDragLeave = useCallback((e) => {
    e.preventDefault()
    setIsDragOver(false)
  }, [])

  const handleDrop = useCallback((e) => {
    e.preventDefault()
    setIsDragOver(false)
    
    const files = Array.from(e.dataTransfer.files)
    files.forEach(file => {
      if (file.name.endsWith('.torrent')) {
        handleTorrentFile(file)
      }
    })
  }, [])

  const handleFileSelect = useCallback((e) => {
    const files = Array.from(e.target.files)
    files.forEach(file => {
      if (file.name.endsWith('.torrent')) {
        handleTorrentFile(file)
      }
    })
  }, [])

  const handleTorrentFile = (file) => {
    // Simulate torrent file processing
    const mockTorrent = {
      id: Math.random().toString(36).substr(2, 9),
      name: file.name.replace('.torrent', ''),
      size: Math.floor(Math.random() * 1000000000) + 100000000, // Random size
      type: 'Archive',
      status: 'downloading',
      progress: Math.floor(Math.random() * 50), // Random initial progress
      speed: Math.floor(Math.random() * 10000000) + 1000000, // Random speed
      date: new Date().toISOString(),
      hash: 'sha256:' + Math.random().toString(36).substr(2, 64),
      peers: Math.floor(Math.random() * 50) + 5,
      seeds: Math.floor(Math.random() * 20) + 1
    }

    dispatch({ type: 'ADD_TRANSFER', payload: mockTorrent })
    addNotification(`Started downloading ${mockTorrent.name}`, 'success')
  }

  const toggleDownload = (transferId) => {
    const transfer = transfers.find(t => t.id === transferId)
    if (!transfer) return

    const newStatus = transfer.status === 'downloading' ? 'paused' : 'downloading'
    dispatch({ 
      type: 'UPDATE_TRANSFER', 
      payload: { id: transferId, status: newStatus }
    })
    
    addNotification(
      `${newStatus === 'paused' ? 'Paused' : 'Resumed'} ${transfer.name}`,
      'info'
    )
  }

  const selectDownloadLocation = () => {
    // In a real app, this would open a folder picker dialog
    addNotification('Folder picker would open here', 'info')
  }

  const getHealthColor = (seeds, peers) => {
    const ratio = seeds / (peers || 1)
    if (ratio > 0.3) return 'text-success'
    if (ratio > 0.1) return 'text-warning'
    return 'text-error'
  }

  const getHealthText = (seeds, peers) => {
    const ratio = seeds / (peers || 1)
    if (ratio > 0.3) return 'Excellent'
    if (ratio > 0.1) return 'Good'
    return 'Poor'
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Download Torrents</h1>
        <p className="text-text-secondary mt-2">
          Download files from the Torrentium network
        </p>
      </div>

      {/* Download Settings */}
      <Card>
        <CardHeader>
          <CardTitle>Download Settings</CardTitle>
          <CardDescription>
            Configure where your downloads are saved
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex space-x-2">
            <Input
              value={downloadLocation}
              onChange={(e) => setDownloadLocation(e.target.value)}
              placeholder="Download location"
              className="flex-1"
            />
            <Button onClick={selectDownloadLocation} variant="outline">
              <FolderOpen className="h-4 w-4 mr-2" />
              Browse
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Torrent Upload Zone */}
      <Card>
        <CardHeader>
          <CardTitle>Add Torrent</CardTitle>
          <CardDescription>
            Drop torrent files here or click to select
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div
            className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
              isDragOver ? 'border-primary bg-primary/10 drag-over' : 'border-surface'
            }`}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            <DownloadIcon className="h-16 w-16 mx-auto mb-4 text-text-secondary" />
            <h3 className="text-lg font-semibold mb-2">
              Drop .torrent files here
            </h3>
            <p className="text-text-secondary mb-4">
              Or click to select torrent files from your computer
            </p>
            <Button onClick={() => fileInputRef.current?.click()}>
              Select Torrent Files
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".torrent"
              multiple
              className="hidden"
              onChange={handleFileSelect}
            />
          </div>
        </CardContent>
      </Card>

      {/* Active Downloads */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Activity className="h-5 w-5" />
            <span>Active Downloads</span>
          </CardTitle>
          <CardDescription>
            Currently downloading files
          </CardDescription>
        </CardHeader>
        <CardContent>
          {activeDownloads.length === 0 ? (
            <div className="text-center py-8 text-text-secondary">
              <DownloadIcon className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No active downloads</p>
              <p className="text-sm">Add torrent files to start downloading</p>
            </div>
          ) : (
            <div className="space-y-4">
              {activeDownloads.map((transfer) => (
                <div key={transfer.id} className="border border-surface rounded-lg p-4">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1 min-w-0">
                      <h4 className="font-medium truncate mb-2">{transfer.name}</h4>
                      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 text-sm text-text-secondary">
                        <div className="flex items-center space-x-2">
                          <Hash className="h-4 w-4" />
                          <span>{formatBytes(transfer.size)}</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <DownloadIcon className="h-4 w-4" />
                          <span>{formatSpeed(transfer.speed)}</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Users className="h-4 w-4" />
                          <span>{transfer.peers || 0} peers</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <AlertCircle className={`h-4 w-4 ${getHealthColor(transfer.seeds || 0, transfer.peers || 0)}`} />
                          <span className={getHealthColor(transfer.seeds || 0, transfer.peers || 0)}>
                            {getHealthText(transfer.seeds || 0, transfer.peers || 0)}
                          </span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2 ml-4">
                      <Badge variant="default">
                        {transfer.status}
                      </Badge>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => toggleDownload(transfer.id)}
                      >
                        {transfer.status === 'downloading' ? (
                          <Pause className="h-4 w-4" />
                        ) : (
                          <Play className="h-4 w-4" />
                        )}
                      </Button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Progress value={transfer.progress} />
                    <div className="flex justify-between text-xs text-text-secondary">
                      <span>{transfer.progress}% complete</span>
                      <span>
                        {transfer.seeds || 0} seeds, {transfer.peers || 0} peers
                      </span>
                    </div>
                  </div>

                  <div className="mt-4 text-xs text-text-secondary">
                    <p>Downloading to: {downloadLocation}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Download Tips */}
      <Card>
        <CardHeader>
          <CardTitle>Download Tips</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2 text-sm text-text-secondary">
            <li>• Files with more seeds download faster</li>
            <li>• Keep the app running to help seed completed downloads</li>
            <li>• Check the health indicator before starting downloads</li>
            <li>• Popular files usually have better availability</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}
