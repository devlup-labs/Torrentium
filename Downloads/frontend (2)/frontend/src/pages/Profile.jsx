import React, { useState } from 'react'
import { 
  User, 
  Camera, 
  Edit3, 
  Save, 
  X, 
  Award, 
  Upload, 
  Download, 
  Shield,
  Copy,
  Settings,
  Trash2
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Avatar, AvatarFallback, AvatarImage } from '../components/ui/Avatar'
import { Badge } from '../components/ui/Badge'
import { badgesList } from '../data/mockData'

export function Profile() {
  const { user, dispatch, addNotification } = useApp()
  const [isEditing, setIsEditing] = useState(false)
  const [editedUsername, setEditedUsername] = useState(user.username)
  const [showSettings, setShowSettings] = useState(false)

  const handleSaveProfile = () => {
    if (!editedUsername.trim()) {
      addNotification('Username cannot be empty', 'error')
      return
    }

    dispatch({ 
      type: 'UPDATE_USER', 
      payload: { username: editedUsername }
    })
    
    setIsEditing(false)
    addNotification('Profile updated successfully', 'success')
  }

  const handleCancelEdit = () => {
    setEditedUsername(user.username)
    setIsEditing(false)
  }

  const copyPeerId = () => {
    navigator.clipboard.writeText(user.peerId)
    addNotification('Peer ID copied to clipboard', 'success')
  }

  const handleAvatarUpload = () => {
    // In a real app, this would open a file picker
    addNotification('Avatar upload would open here', 'info')
  }

  const getTrustScoreColor = (score) => {
    if (score >= 90) return 'text-success'
    if (score >= 70) return 'text-warning'
    return 'text-error'
  }

  const getTrustScoreDescription = (score) => {
    if (score >= 90) return 'Excellent reputation'
    if (score >= 70) return 'Good reputation'
    if (score >= 50) return 'Average reputation'
    return 'Building reputation'
  }

  const getInitials = (username) => {
    return username.split(/\s+/).map(word => word[0]).join('').toUpperCase().slice(0, 2)
  }

  const userBadges = user.badges || []
  const availableBadges = badgesList.filter(badge => !userBadges.includes(badge))

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Profile</h1>
        <p className="text-text-secondary mt-2">
          Manage your account and view your achievements
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Profile Information */}
        <div className="lg:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <User className="h-5 w-5" />
                <span>Profile Information</span>
              </CardTitle>
              <CardDescription>
                Your public profile information
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-6">
                {/* Avatar and Username */}
                <div className="flex items-center space-x-6">
                  <div className="relative">
                    <Avatar className="h-24 w-24">
                      <AvatarImage src={user.avatar} />
                      <AvatarFallback className="text-2xl">
                        {getInitials(user.username)}
                      </AvatarFallback>
                    </Avatar>
                    <Button
                      size="icon"
                      className="absolute -bottom-2 -right-2 h-8 w-8"
                      onClick={handleAvatarUpload}
                    >
                      <Camera className="h-4 w-4" />
                    </Button>
                  </div>
                  
                  <div className="flex-1">
                    {isEditing ? (
                      <div className="space-y-3">
                        <Input
                          value={editedUsername}
                          onChange={(e) => setEditedUsername(e.target.value)}
                          placeholder="Enter username"
                          className="text-lg font-semibold"
                        />
                        <div className="flex space-x-2">
                          <Button size="sm" onClick={handleSaveProfile}>
                            <Save className="h-4 w-4 mr-1" />
                            Save
                          </Button>
                          <Button size="sm" variant="outline" onClick={handleCancelEdit}>
                            <X className="h-4 w-4 mr-1" />
                            Cancel
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <div className="flex items-center space-x-2">
                          <h2 className="text-2xl font-bold">{user.username}</h2>
                          <Button
                            size="icon"
                            variant="ghost"
                            onClick={() => setIsEditing(true)}
                          >
                            <Edit3 className="h-4 w-4" />
                          </Button>
                        </div>
                        <div className="flex items-center space-x-2">
                          <Shield className={`h-5 w-5 ${getTrustScoreColor(user.trustScore)}`} />
                          <span className={`font-semibold ${getTrustScoreColor(user.trustScore)}`}>
                            Trust Score: {user.trustScore}
                          </span>
                          <Badge variant={user.trustScore >= 80 ? 'success' : user.trustScore >= 60 ? 'warning' : 'destructive'}>
                            {getTrustScoreDescription(user.trustScore)}
                          </Badge>
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                {/* Peer ID */}
                <div className="space-y-2">
                  <label className="text-sm font-medium">Peer ID</label>
                  <div className="flex space-x-2">
                    <Input
                      value={user.peerId}
                      readOnly
                      className="font-mono text-sm"
                    />
                    <Button variant="outline" onClick={copyPeerId}>
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                  <p className="text-xs text-text-secondary">
                    Your unique identifier on the Torrentium network
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Statistics */}
          <Card>
            <CardHeader>
              <CardTitle>Statistics</CardTitle>
              <CardDescription>
                Your activity and contribution to the network
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <div className="text-center space-y-2">
                  <Upload className="h-8 w-8 mx-auto text-success" />
                  <div className="text-2xl font-bold text-success">{user.totalShared}</div>
                  <p className="text-sm text-text-secondary">Files Shared</p>
                </div>
                
                <div className="text-center space-y-2">
                  <Download className="h-8 w-8 mx-auto text-primary" />
                  <div className="text-2xl font-bold text-primary">{user.totalDownloaded}</div>
                  <p className="text-sm text-text-secondary">Files Downloaded</p>
                </div>
                
                <div className="text-center space-y-2">
                  <Award className="h-8 w-8 mx-auto text-warning" />
                  <div className="text-2xl font-bold text-warning">{userBadges.length}</div>
                  <p className="text-sm text-text-secondary">Badges Earned</p>
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Account Settings */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <Settings className="h-5 w-5" />
                <span>Account Settings</span>
              </CardTitle>
              <CardDescription>
                Manage your account preferences
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-medium">Notifications</h4>
                    <p className="text-sm text-text-secondary">
                      Receive notifications for downloads and network activity
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Enabled
                  </Button>
                </div>
                
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-medium">Auto-start downloads</h4>
                    <p className="text-sm text-text-secondary">
                      Automatically start downloading when torrents are added
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Enabled
                  </Button>
                </div>
                
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-medium">Share statistics</h4>
                    <p className="text-sm text-text-secondary">
                      Allow your statistics to be visible to other users
                    </p>
                  </div>
                  <Button variant="outline" size="sm">
                    Enabled
                  </Button>
                </div>

                <div className="pt-4 border-t border-surface">
                  <Button variant="destructive" size="sm">
                    <Trash2 className="h-4 w-4 mr-2" />
                    Delete Account
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Badges */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <Award className="h-5 w-5" />
                <span>Achievements</span>
              </CardTitle>
              <CardDescription>
                Badges you've earned on the network
              </CardDescription>
            </CardHeader>
            <CardContent>
              {userBadges.length === 0 ? (
                <div className="text-center py-4 text-text-secondary">
                  <Award className="h-8 w-8 mx-auto mb-2 opacity-50" />
                  <p className="text-sm">No badges yet</p>
                  <p className="text-xs">Start sharing files to earn badges</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {userBadges.map((badge, index) => (
                    <div key={index} className="flex items-center space-x-2 p-2 rounded-lg bg-background">
                      <Award className="h-4 w-4 text-warning" />
                      <span className="text-sm font-medium">{badge}</span>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Available Badges */}
          <Card>
            <CardHeader>
              <CardTitle>Available Badges</CardTitle>
              <CardDescription>
                Badges you can earn by being active
              </CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-2 max-h-64 overflow-y-auto">
                {availableBadges.slice(0, 10).map((badge, index) => (
                  <div key={index} className="flex items-center space-x-2 p-2 rounded-lg bg-surface/50">
                    <Award className="h-4 w-4 text-text-secondary" />
                    <span className="text-sm text-text-secondary">{badge}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Quick Stats */}
          <Card>
            <CardHeader>
              <CardTitle>Quick Stats</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Member since</span>
                  <span>Jan 2024</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Network rank</span>
                  <span className="text-warning">#5</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Upload ratio</span>
                  <span className="text-success">1.75</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-text-secondary">Data shared</span>
                  <span>2.3 TB</span>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
