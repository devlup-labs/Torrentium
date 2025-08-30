import React, { useState, useMemo } from 'react'
import { 
  Search, 
  Filter, 
  Play, 
  Pause, 
  RotateCcw, 
  Trash2,
  ChevronDown,
  Download,
  Upload,
  CheckCircle,
  XCircle
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Badge } from '../components/ui/Badge'
import { Progress } from '../components/ui/Progress'
import { formatBytes, formatSpeed, formatDate } from '../lib/utils'

const statusFilters = [
  { value: 'all', label: 'All Transfers' },
  { value: 'completed', label: 'Completed' },
  { value: 'downloading', label: 'Downloading' },
  { value: 'uploading', label: 'Uploading' },
  { value: 'failed', label: 'Failed' },
  { value: 'paused', label: 'Paused' }
]

export function History() {
  const { transfers, dispatch, addNotification } = useApp()
  const [searchTerm, setSearchTerm] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [sortBy, setSortBy] = useState('date')
  const [sortOrder, setSortOrder] = useState('desc')

  const filteredAndSortedTransfers = useMemo(() => {
    let filtered = transfers

    // Apply search filter
    if (searchTerm) {
      filtered = filtered.filter(transfer =>
        transfer.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        transfer.type.toLowerCase().includes(searchTerm.toLowerCase())
      )
    }

    // Apply status filter
    if (statusFilter !== 'all') {
      filtered = filtered.filter(transfer => transfer.status === statusFilter)
    }

    // Apply sorting
    filtered.sort((a, b) => {
      let aValue, bValue

      switch (sortBy) {
        case 'name':
          aValue = a.name.toLowerCase()
          bValue = b.name.toLowerCase()
          break
        case 'size':
          aValue = a.size
          bValue = b.size
          break
        case 'progress':
          aValue = a.progress
          bValue = b.progress
          break
        case 'speed':
          aValue = a.speed
          bValue = b.speed
          break
        case 'date':
        default:
          aValue = new Date(a.date)
          bValue = new Date(b.date)
          break
      }

      if (sortOrder === 'asc') {
        return aValue < bValue ? -1 : aValue > bValue ? 1 : 0
      } else {
        return aValue > bValue ? -1 : aValue < bValue ? 1 : 0
      }
    })

    return filtered
  }, [transfers, searchTerm, statusFilter, sortBy, sortOrder])

  const getStatusColor = (status) => {
    switch (status) {
      case 'completed': return 'success'
      case 'downloading': return 'default'
      case 'uploading': return 'secondary'
      case 'failed': return 'destructive'
      case 'paused': return 'outline'
      default: return 'outline'
    }
  }

  const getStatusIcon = (status) => {
    switch (status) {
      case 'completed': return CheckCircle
      case 'downloading': return Download
      case 'uploading': return Upload
      case 'failed': return XCircle
      default: return null
    }
  }

  const toggleTransfer = (transferId) => {
    const transfer = transfers.find(t => t.id === transferId)
    if (!transfer) return

    if (transfer.status === 'downloading' || transfer.status === 'uploading') {
      dispatch({ 
        type: 'UPDATE_TRANSFER', 
        payload: { id: transferId, status: 'paused' }
      })
      addNotification(`Paused ${transfer.name}`, 'info')
    } else if (transfer.status === 'paused') {
      const newStatus = transfer.progress < 100 ? 'downloading' : 'uploading'
      dispatch({ 
        type: 'UPDATE_TRANSFER', 
        payload: { id: transferId, status: newStatus }
      })
      addNotification(`Resumed ${transfer.name}`, 'info')
    }
  }

  const retryTransfer = (transferId) => {
    const transfer = transfers.find(t => t.id === transferId)
    if (!transfer) return

    dispatch({ 
      type: 'UPDATE_TRANSFER', 
      payload: { 
        id: transferId, 
        status: 'downloading', 
        progress: 0 
      }
    })
    addNotification(`Retrying ${transfer.name}`, 'info')
  }

  const deleteTransfer = (transferId) => {
    const transfer = transfers.find(t => t.id === transferId)
    if (!transfer) return

    dispatch({ type: 'DELETE_TRANSFER', payload: transferId })
    addNotification(`Removed ${transfer.name} from history`, 'success')
  }

  const handleSort = (field) => {
    if (sortBy === field) {
      setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc')
    } else {
      setSortBy(field)
      setSortOrder('desc')
    }
  }

  const getSortIcon = (field) => {
    if (sortBy !== field) return null
    return (
      <ChevronDown 
        className={`h-4 w-4 ml-1 transition-transform ${
          sortOrder === 'asc' ? 'rotate-180' : ''
        }`} 
      />
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Transfer History</h1>
        <p className="text-text-secondary mt-2">
          View and manage all your downloads and uploads
        </p>
      </div>

      {/* Filters and Search */}
      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>
            Search and filter your transfer history
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col md:flex-row gap-4">
            {/* Search */}
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-text-secondary" />
                <Input
                  placeholder="Search transfers..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>

            {/* Status Filter */}
            <div className="md:w-48">
              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="w-full h-10 px-3 rounded-md border border-surface bg-background text-sm"
              >
                {statusFilters.map(filter => (
                  <option key={filter.value} value={filter.value}>
                    {filter.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Transfer Statistics */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="text-2xl font-bold text-success">
              {transfers.filter(t => t.status === 'completed').length}
            </div>
            <p className="text-xs text-text-secondary">Completed</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-2xl font-bold text-primary">
              {transfers.filter(t => t.status === 'downloading').length}
            </div>
            <p className="text-xs text-text-secondary">Downloading</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-2xl font-bold text-secondary">
              {transfers.filter(t => t.status === 'uploading').length}
            </div>
            <p className="text-xs text-text-secondary">Uploading</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="text-2xl font-bold text-error">
              {transfers.filter(t => t.status === 'failed').length}
            </div>
            <p className="text-xs text-text-secondary">Failed</p>
          </CardContent>
        </Card>
      </div>

      {/* Transfer List */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>Transfer History</CardTitle>
              <CardDescription>
                {filteredAndSortedTransfers.length} of {transfers.length} transfers
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {filteredAndSortedTransfers.length === 0 ? (
            <div className="text-center py-8 text-text-secondary">
              <Filter className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No transfers found</p>
              <p className="text-sm">Try adjusting your search or filters</p>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Table Header */}
              <div className="hidden md:grid grid-cols-12 gap-4 pb-2 border-b border-surface text-sm font-medium text-text-secondary">
                <div 
                  className="col-span-4 cursor-pointer flex items-center hover:text-text"
                  onClick={() => handleSort('name')}
                >
                  Name {getSortIcon('name')}
                </div>
                <div 
                  className="col-span-2 cursor-pointer flex items-center hover:text-text"
                  onClick={() => handleSort('size')}
                >
                  Size {getSortIcon('size')}
                </div>
                <div 
                  className="col-span-2 cursor-pointer flex items-center hover:text-text"
                  onClick={() => handleSort('progress')}
                >
                  Progress {getSortIcon('progress')}
                </div>
                <div 
                  className="col-span-2 cursor-pointer flex items-center hover:text-text"
                  onClick={() => handleSort('speed')}
                >
                  Speed {getSortIcon('speed')}
                </div>
                <div 
                  className="col-span-2 cursor-pointer flex items-center hover:text-text"
                  onClick={() => handleSort('date')}
                >
                  Date {getSortIcon('date')}
                </div>
              </div>

              {/* Transfer Items */}
              {filteredAndSortedTransfers.map((transfer) => {
                const StatusIcon = getStatusIcon(transfer.status)
                
                return (
                  <div key={transfer.id} className="border border-surface rounded-lg p-4">
                    <div className="md:grid md:grid-cols-12 md:gap-4 md:items-center space-y-2 md:space-y-0">
                      {/* Name and Type */}
                      <div className="md:col-span-4">
                        <div className="flex items-center space-x-2">
                          {StatusIcon && <StatusIcon className="h-4 w-4 flex-shrink-0" />}
                          <div className="min-w-0 flex-1">
                            <p className="font-medium truncate">{transfer.name}</p>
                            <p className="text-xs text-text-secondary">{transfer.type}</p>
                          </div>
                        </div>
                      </div>

                      {/* Size */}
                      <div className="md:col-span-2">
                        <span className="text-sm">{formatBytes(transfer.size)}</span>
                      </div>

                      {/* Progress */}
                      <div className="md:col-span-2">
                        <div className="space-y-1">
                          <Progress value={transfer.progress} className="h-2" />
                          <div className="flex items-center justify-between">
                            <span className="text-xs text-text-secondary">
                              {transfer.progress}%
                            </span>
                            <Badge variant={getStatusColor(transfer.status)} className="text-xs">
                              {transfer.status}
                            </Badge>
                          </div>
                        </div>
                      </div>

                      {/* Speed */}
                      <div className="md:col-span-2">
                        <span className="text-sm">
                          {transfer.speed > 0 ? formatSpeed(transfer.speed) : '-'}
                        </span>
                      </div>

                      {/* Date and Actions */}
                      <div className="md:col-span-2">
                        <div className="flex items-center justify-between">
                          <span className="text-xs text-text-secondary">
                            {formatDate(transfer.date)}
                          </span>
                          <div className="flex space-x-1">
                            {(transfer.status === 'downloading' || transfer.status === 'uploading' || transfer.status === 'paused') && (
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => toggleTransfer(transfer.id)}
                                className="h-6 w-6"
                              >
                                {transfer.status === 'paused' ? (
                                  <Play className="h-3 w-3" />
                                ) : (
                                  <Pause className="h-3 w-3" />
                                )}
                              </Button>
                            )}
                            {transfer.status === 'failed' && (
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => retryTransfer(transfer.id)}
                                className="h-6 w-6"
                              >
                                <RotateCcw className="h-3 w-3" />
                              </Button>
                            )}
                            <Button
                              variant="ghost"
                              size="icon"
                              onClick={() => deleteTransfer(transfer.id)}
                              className="h-6 w-6 text-error hover:text-error"
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
