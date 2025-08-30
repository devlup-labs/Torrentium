import React, { useState } from 'react'
import { 
  Users, 
  UserPlus, 
  Shield, 
  ShieldCheck, 
  Clock, 
  Ban, 
  CheckCircle,
  XCircle,
  Copy,
  QrCode,
  Settings
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Avatar, AvatarFallback, AvatarImage } from '../components/ui/Avatar'
import { Badge } from '../components/ui/Badge'
import { formatDate, generateShareCode } from '../lib/utils'

export function PrivateNetwork() {
  const { peers, user, dispatch, addNotification } = useApp()
  const [inviteCode, setInviteCode] = useState('')
  const [showInviteModal, setShowInviteModal] = useState(false)
  const [generatedCode, setGeneratedCode] = useState('')

  const onlinePeers = peers.filter(p => p.isOnline && !p.isBlocked)
  const offlinePeers = peers.filter(p => !p.isOnline && !p.isBlocked)
  const blockedPeers = peers.filter(p => p.isBlocked)

  const generateInviteCode = () => {
    const code = generateShareCode()
    setGeneratedCode(code)
    setShowInviteModal(true)
    addNotification('Invite code generated successfully', 'success')
  }

  const copyInviteCode = () => {
    navigator.clipboard.writeText(generatedCode)
    addNotification('Invite code copied to clipboard', 'success')
  }

  const addPeerByCode = () => {
    if (!inviteCode.trim()) {
      addNotification('Please enter an invite code', 'error')
      return
    }

    // Simulate adding peer
    const newPeer = {
      id: `peer_${Date.now()}`,
      username: `User_${inviteCode.slice(0, 4)}`,
      avatar: null,
      trustScore: Math.floor(Math.random() * 50) + 50,
      lastSeen: new Date().toISOString(),
      isOnline: Math.random() > 0.5,
      isBlocked: false
    }

    dispatch({ type: 'ADD_PEER', payload: newPeer })
    addNotification(`Successfully added ${newPeer.username} to your network`, 'success')
    setInviteCode('')
  }

  const blockPeer = (peerId) => {
    const peer = peers.find(p => p.id === peerId)
    if (!peer) return

    dispatch({ type: 'BLOCK_PEER', payload: peerId })
    addNotification(`Blocked ${peer.username}`, 'warning')
  }

  const unblockPeer = (peerId) => {
    const peer = peers.find(p => p.id === peerId)
    if (!peer) return

    dispatch({ type: 'UNBLOCK_PEER', payload: peerId })
    addNotification(`Unblocked ${peer.username}`, 'success')
  }

  const getTrustScoreColor = (score) => {
    if (score >= 90) return 'text-success'
    if (score >= 70) return 'text-warning'
    return 'text-error'
  }

  const getTrustScoreBadge = (score) => {
    if (score >= 90) return 'success'
    if (score >= 70) return 'warning'
    return 'destructive'
  }

  const getInitials = (username) => {
    return username.split('_').map(part => part[0]).join('').toUpperCase()
  }

  const PeerCard = ({ peer }) => (
    <div className="flex items-center space-x-4 p-4 border border-surface rounded-lg">
      <Avatar className="h-12 w-12">
        <AvatarImage src={peer.avatar} />
        <AvatarFallback>{getInitials(peer.username)}</AvatarFallback>
      </Avatar>
      
      <div className="flex-1 min-w-0">
        <div className="flex items-center space-x-2 mb-1">
          <h4 className="font-medium truncate">{peer.username}</h4>
          {peer.isOnline ? (
            <div className="flex items-center space-x-1">
              <div className="w-2 h-2 bg-success rounded-full"></div>
              <span className="text-xs text-success">Online</span>
            </div>
          ) : (
            <div className="flex items-center space-x-1">
              <div className="w-2 h-2 bg-text-secondary rounded-full"></div>
              <span className="text-xs text-text-secondary">Offline</span>
            </div>
          )}
        </div>
        
        <div className="flex items-center space-x-4 text-sm text-text-secondary">
          <div className="flex items-center space-x-1">
            <Shield className="h-4 w-4" />
            <span className={getTrustScoreColor(peer.trustScore)}>
              {peer.trustScore}
            </span>
          </div>
          <div className="flex items-center space-x-1">
            <Clock className="h-4 w-4" />
            <span>{formatDate(peer.lastSeen)}</span>
          </div>
        </div>
      </div>

      <div className="flex items-center space-x-2">
        <Badge variant={getTrustScoreBadge(peer.trustScore)}>
          Trust: {peer.trustScore}
        </Badge>
        {peer.isBlocked ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => unblockPeer(peer.id)}
          >
            <CheckCircle className="h-4 w-4 mr-1" />
            Unblock
          </Button>
        ) : (
          <Button
            variant="destructive"
            size="sm"
            onClick={() => blockPeer(peer.id)}
          >
            <Ban className="h-4 w-4 mr-1" />
            Block
          </Button>
        )}
      </div>
    </div>
  )

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Private Network</h1>
        <p className="text-text-secondary mt-2">
          Manage your trusted peers and network connections
        </p>
      </div>

      {/* Network Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-2">
              <Users className="h-5 w-5 text-primary" />
              <div>
                <div className="text-2xl font-bold">{peers.filter(p => !p.isBlocked).length}</div>
                <p className="text-xs text-text-secondary">Total Peers</p>
              </div>
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-2">
              <CheckCircle className="h-5 w-5 text-success" />
              <div>
                <div className="text-2xl font-bold text-success">{onlinePeers.length}</div>
                <p className="text-xs text-text-secondary">Online Now</p>
              </div>
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-2">
              <ShieldCheck className="h-5 w-5 text-warning" />
              <div>
                <div className="text-2xl font-bold text-warning">{user.trustScore}</div>
                <p className="text-xs text-text-secondary">Your Trust Score</p>
              </div>
            </div>
          </CardContent>
        </Card>
        
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center space-x-2">
              <XCircle className="h-5 w-5 text-error" />
              <div>
                <div className="text-2xl font-bold text-error">{blockedPeers.length}</div>
                <p className="text-xs text-text-secondary">Blocked Peers</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Add Peer Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <UserPlus className="h-5 w-5" />
              <span>Add New Peer</span>
            </CardTitle>
            <CardDescription>
              Enter an invite code to connect with a new peer
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex space-x-2">
              <Input
                placeholder="Enter invite code"
                value={inviteCode}
                onChange={(e) => setInviteCode(e.target.value)}
                className="flex-1"
              />
              <Button onClick={addPeerByCode}>
                Add Peer
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <QrCode className="h-5 w-5" />
              <span>Invite Others</span>
            </CardTitle>
            <CardDescription>
              Generate an invite code to share with others
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <Button onClick={generateInviteCode} className="w-full">
                Generate Invite Code
              </Button>
              {generatedCode && (
                <div className="flex space-x-2">
                  <Input
                    value={generatedCode}
                    readOnly
                    className="flex-1 font-mono"
                  />
                  <Button
                    variant="outline"
                    onClick={copyInviteCode}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Online Peers */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <CheckCircle className="h-5 w-5 text-success" />
            <span>Online Peers ({onlinePeers.length})</span>
          </CardTitle>
          <CardDescription>
            Peers currently active on the network
          </CardDescription>
        </CardHeader>
        <CardContent>
          {onlinePeers.length === 0 ? (
            <div className="text-center py-8 text-text-secondary">
              <Users className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No peers online</p>
              <p className="text-sm">Invite others to join your network</p>
            </div>
          ) : (
            <div className="space-y-4">
              {onlinePeers.map((peer) => (
                <PeerCard key={peer.id} peer={peer} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Offline Peers */}
      {offlinePeers.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <Clock className="h-5 w-5 text-text-secondary" />
              <span>Offline Peers ({offlinePeers.length})</span>
            </CardTitle>
            <CardDescription>
              Peers not currently active
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {offlinePeers.map((peer) => (
                <PeerCard key={peer.id} peer={peer} />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Blocked Peers */}
      {blockedPeers.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <Ban className="h-5 w-5 text-error" />
              <span>Blocked Peers ({blockedPeers.length})</span>
            </CardTitle>
            <CardDescription>
              Peers you have blocked from your network
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {blockedPeers.map((peer) => (
                <PeerCard key={peer.id} peer={peer} />
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Network Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Settings className="h-5 w-5" />
            <span>Network Settings</span>
          </CardTitle>
          <CardDescription>
            Configure your network preferences
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Auto-accept trusted peers</h4>
                <p className="text-sm text-text-secondary">
                  Automatically accept connections from peers with trust score {'>'}80
                </p>
              </div>
              <Button variant="outline" size="sm">
                Enabled
              </Button>
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Share with network</h4>
                <p className="text-sm text-text-secondary">
                  Allow network peers to download your shared files
                </p>
              </div>
              <Button variant="outline" size="sm">
                Enabled
              </Button>
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Discovery mode</h4>
                <p className="text-sm text-text-secondary">
                  Allow others to discover and invite you to their networks
                </p>
              </div>
              <Button variant="outline" size="sm">
                Enabled
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
