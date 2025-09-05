import React, { useState, useRef, useCallback } from 'react'
import { 
  Upload as UploadIcon, 
  File, 
  X, 
  CheckCircle,
  Hash,
  HardDrive
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Progress } from '../components/ui/Progress'
import { Badge } from '../components/ui/Badge'
import { formatBytes } from '../lib/utils'

export function Upload() {
  const { dispatch, addNotification } = useApp()
  const [uploadFiles, setUploadFiles] = useState([])
  const [isDragOver, setIsDragOver] = useState(false)
  const fileInputRef = useRef(null)

  const generateFileHash = (file) => {
    // Simulate SHA-256 hash generation
    return 'sha256:' + Math.random().toString(36).substr(2, 64)
  }

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
    handleFiles(files)
  }, [])

  const handleFileSelect = useCallback((e) => {
    const files = Array.from(e.target.files)
    handleFiles(files)
  }, [])

  const handleFiles = (files) => {
    const newFiles = files.map(file => ({
      id: Math.random().toString(36).substr(2, 9),
      file,
      name: file.name,
      size: file.size,
      type: file.type || 'application/octet-stream',
      hash: generateFileHash(file),
      progress: 0,
      status: 'pending' // pending, uploading, completed, failed
    }))

    setUploadFiles(prev => [...prev, ...newFiles])
    addNotification(`Added ${files.length} file(s) to upload queue`, 'success')
  }

  const removeFile = (fileId) => {
    setUploadFiles(prev => prev.filter(f => f.id !== fileId))
  }

  const uploadFile = async (fileItem) => {
    // Simulate upload process
    setUploadFiles(prev => 
      prev.map(f => 
        f.id === fileItem.id 
          ? { ...f, status: 'uploading', progress: 0 }
          : f
      )
    )

    // Simulate upload progress
    for (let progress = 0; progress <= 100; progress += 10) {
      await new Promise(resolve => setTimeout(resolve, 200))
      setUploadFiles(prev => 
        prev.map(f => 
          f.id === fileItem.id 
            ? { ...f, progress }
            : f
        )
      )
    }

    // Complete upload
    setUploadFiles(prev => 
      prev.map(f => 
        f.id === fileItem.id 
          ? { ...f, status: 'completed', progress: 100 }
          : f
      )
    )

    // Add to transfers
    const transfer = {
      id: fileItem.id,
      name: fileItem.name,
      size: fileItem.size,
      type: fileItem.type,
      status: 'uploading',
      progress: 100,
      speed: 0,
      date: new Date().toISOString(),
      hash: fileItem.hash
    }

    dispatch({ type: 'ADD_TRANSFER', payload: transfer })
    addNotification(`Successfully uploaded ${fileItem.name}`, 'success')
  }

  const uploadAll = async () => {
    const pendingFiles = uploadFiles.filter(f => f.status === 'pending')
    
    for (const file of pendingFiles) {
      await uploadFile(file)
    }
  }

  const clearCompleted = () => {
    setUploadFiles(prev => prev.filter(f => f.status !== 'completed'))
  }

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return 'success'
      case 'uploading': return 'default'
      case 'failed': return 'destructive'
      default: return 'outline'
    }
  }

  const getFileIcon = (type) => {
    if (type.startsWith('image/')) return '🖼️'
    if (type.startsWith('video/')) return '🎥'
    if (type.startsWith('audio/')) return '🎵'
    if (type.includes('pdf')) return '📄'
    if (type.includes('zip') || type.includes('rar')) return '📦'
    return '📄'
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Upload Files</h1>
        <p className="text-text-secondary mt-2">
          Share files with the Torrentium network
        </p>
      </div>

      {/* Upload Zone */}
      <Card>
        <CardHeader>
          <CardTitle>File Upload</CardTitle>
          <CardDescription>
            Drag and drop files here or click to select files
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
            <UploadIcon className="h-16 w-16 mx-auto mb-4 text-text-secondary" />
            <h3 className="text-lg font-semibold mb-2">
              Drop files here to upload
            </h3>
            <p className="text-text-secondary mb-4">
              Or click the button below to select files
            </p>
            <Button onClick={() => fileInputRef.current?.click()}>
              Select Files
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={handleFileSelect}
            />
          </div>
        </CardContent>
      </Card>

      {/* Upload Queue */}
      {uploadFiles.length > 0 && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Upload Queue</CardTitle>
                <CardDescription>
                  {uploadFiles.length} file(s) ready for upload
                </CardDescription>
              </div>
              <div className="flex space-x-2">
                <Button 
                  onClick={uploadAll}
                  disabled={!uploadFiles.some(f => f.status === 'pending')}
                >
                  Upload All
                </Button>
                <Button 
                  variant="outline" 
                  onClick={clearCompleted}
                  disabled={!uploadFiles.some(f => f.status === 'completed')}
                >
                  Clear Completed
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {uploadFiles.map((fileItem) => (
                <div key={fileItem.id} className="border border-surface rounded-lg p-4">
                  <div className="flex items-start space-x-4">
                    <div className="text-2xl">
                      {getFileIcon(fileItem.type)}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-2">
                        <h4 className="font-medium truncate">{fileItem.name}</h4>
                        <div className="flex items-center space-x-2">
                          <Badge variant={getStatusColor(fileItem.status)}>
                            {fileItem.status}
                          </Badge>
                          {fileItem.status === 'pending' && (
                            <Button
                              size="sm"
                              onClick={() => uploadFile(fileItem)}
                            >
                              Upload
                            </Button>
                          )}
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => removeFile(fileItem.id)}
                          >
                            <X className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                      
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm text-text-secondary mb-3">
                        <div className="flex items-center space-x-2">
                          <HardDrive className="h-4 w-4" />
                          <span>{formatBytes(fileItem.size)}</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <File className="h-4 w-4" />
                          <span>{fileItem.type}</span>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Hash className="h-4 w-4" />
                          <span className="truncate font-mono text-xs">
                            {fileItem.hash}
                          </span>
                        </div>
                      </div>

                      {fileItem.status === 'uploading' && (
                        <div className="space-y-2">
                          <Progress value={fileItem.progress} />
                          <div className="flex justify-between text-xs text-text-secondary">
                            <span>{fileItem.progress}% uploaded</span>
                            <span>Uploading...</span>
                          </div>
                        </div>
                      )}

                      {fileItem.status === 'completed' && (
                        <div className="flex items-center space-x-2 text-success">
                          <CheckCircle className="h-4 w-4" />
                          <span className="text-sm">Upload completed successfully</span>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Upload Tips */}
      <Card>
        <CardHeader>
          <CardTitle>Upload Tips</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2 text-sm text-text-secondary">
            <li>• Files are automatically shared with the network once uploaded</li>
            <li>• Larger files may take longer to generate hash checksums</li>
            <li>• Keep your application running to help seed uploaded files</li>
            <li>• Popular files increase your trust score faster</li>
          </ul>
        </CardContent>
      </Card>
    </div>
  )
}
