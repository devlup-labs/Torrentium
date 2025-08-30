import React, { useState, useMemo } from 'react'
import { 
  Trophy, 
  Medal, 
  Award, 
  Crown, 
  TrendingUp,
  Filter,
  Search,
  Star
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { Avatar, AvatarFallback, AvatarImage } from '../components/ui/Avatar'
import { Badge } from '../components/ui/Badge'
import { mockLeaderboard, badgesList } from '../data/mockData'

const categoryFilters = [
  { value: 'all', label: 'All Categories' },
  { value: 'badges', label: 'Most Badges' },
  { value: 'trust', label: 'Highest Trust' },
  { value: 'shared', label: 'Most Shared' },
  { value: 'active', label: 'Most Active' }
]

export function Leaderboard() {
  const { user } = useApp()
  const [searchTerm, setSearchTerm] = useState('')
  const [categoryFilter, setCategoryFilter] = useState('all')

  const filteredLeaderboard = useMemo(() => {
    let filtered = [...mockLeaderboard]

    // Apply search filter
    if (searchTerm) {
      filtered = filtered.filter(user =>
        user.username.toLowerCase().includes(searchTerm.toLowerCase())
      )
    }

    // Apply category sorting
    switch (categoryFilter) {
      case 'trust':
        filtered.sort((a, b) => b.trustScore - a.trustScore)
        break
      case 'shared':
        filtered.sort((a, b) => b.totalShared - a.totalShared)
        break
      case 'badges':
      default:
        filtered.sort((a, b) => b.badgeCount - a.badgeCount)
        break
    }

    // Update ranks
    filtered.forEach((user, index) => {
      user.rank = index + 1
    })

    return filtered
  }, [searchTerm, categoryFilter])

  const getRankIcon = (rank) => {
    switch (rank) {
      case 1:
        return <Crown className="h-6 w-6 text-yellow-500" />
      case 2:
        return <Medal className="h-6 w-6 text-gray-400" />
      case 3:
        return <Award className="h-6 w-6 text-amber-600" />
      default:
        return (
          <div className="h-6 w-6 rounded-full bg-surface flex items-center justify-center text-xs font-bold">
            {rank}
          </div>
        )
    }
  }

  const getRankColor = (rank) => {
    switch (rank) {
      case 1: return 'bg-gradient-to-r from-yellow-500/20 to-yellow-600/20 border-yellow-500/30'
      case 2: return 'bg-gradient-to-r from-gray-400/20 to-gray-500/20 border-gray-400/30'
      case 3: return 'bg-gradient-to-r from-amber-600/20 to-amber-700/20 border-amber-600/30'
      default: return 'bg-surface border-surface'
    }
  }

  const getInitials = (username) => {
    return username.split(/\s+/).map(word => word[0]).join('').toUpperCase().slice(0, 2)
  }

  const getTrustScoreColor = (score) => {
    if (score >= 90) return 'text-success'
    if (score >= 70) return 'text-warning'
    return 'text-error'
  }

  const currentUserPosition = filteredLeaderboard.findIndex(u => u.username === user.username)

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Leaderboard</h1>
        <p className="text-text-secondary mt-2">
          Top contributors to the Torrentium network
        </p>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>
            Search users and filter by categories
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col md:flex-row gap-4">
            {/* Search */}
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-text-secondary" />
                <Input
                  placeholder="Search users..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>

            {/* Category Filter */}
            <div className="md:w-48">
              <select
                value={categoryFilter}
                onChange={(e) => setCategoryFilter(e.target.value)}
                className="w-full h-10 px-3 rounded-md border border-surface bg-background text-sm"
              >
                {categoryFilters.map(filter => (
                  <option key={filter.value} value={filter.value}>
                    {filter.label}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Top 3 Podium */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center space-x-2">
            <Trophy className="h-5 w-5 text-warning" />
            <span>Top Contributors</span>
          </CardTitle>
          <CardDescription>
            The highest-ranking users on the network
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {filteredLeaderboard.slice(0, 3).map((user, index) => (
              <div
                key={user.id}
                className={`p-6 rounded-lg border text-center ${getRankColor(index + 1)}`}
              >
                <div className="flex justify-center mb-4">
                  {getRankIcon(index + 1)}
                </div>
                
                <Avatar className="h-16 w-16 mx-auto mb-4">
                  <AvatarImage src={user.avatar} />
                  <AvatarFallback className="text-lg">
                    {getInitials(user.username)}
                  </AvatarFallback>
                </Avatar>
                
                <h3 className="font-bold text-lg mb-2">{user.username}</h3>
                
                <div className="space-y-2">
                  <div className="flex items-center justify-center space-x-2">
                    <Award className="h-4 w-4 text-warning" />
                    <span className="text-sm font-medium">{user.badgeCount} badges</span>
                  </div>
                  
                  <div className="flex items-center justify-center space-x-2">
                    <TrendingUp className={`h-4 w-4 ${getTrustScoreColor(user.trustScore)}`} />
                    <span className={`text-sm ${getTrustScoreColor(user.trustScore)}`}>
                      {user.trustScore} trust
                    </span>
                  </div>
                  
                  <div className="text-xs text-text-secondary">
                    {user.totalShared} files shared
                  </div>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Current User Position */}
      {currentUserPosition >= 3 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center space-x-2">
              <Star className="h-5 w-5 text-primary" />
              <span>Your Position</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center space-x-4 p-4 bg-primary/10 rounded-lg border border-primary/20">
              <div className="flex items-center space-x-2">
                {getRankIcon(currentUserPosition + 1)}
                <span className="font-bold text-lg">#{currentUserPosition + 1}</span>
              </div>
              
              <Avatar className="h-12 w-12">
                <AvatarImage src={user.avatar} />
                <AvatarFallback>
                  {getInitials(user.username)}
                </AvatarFallback>
              </Avatar>
              
              <div className="flex-1">
                <h4 className="font-medium">{user.username}</h4>
                <div className="flex items-center space-x-4 text-sm text-text-secondary">
                  <span>{user.badges?.length || 0} badges</span>
                  <span>Trust: {user.trustScore}</span>
                  <span>{user.totalShared} shared</span>
                </div>
              </div>
              
              <Badge variant="secondary">You</Badge>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Full Leaderboard */}
      <Card>
        <CardHeader>
          <CardTitle>Full Rankings</CardTitle>
          <CardDescription>
            Complete leaderboard of all network users
          </CardDescription>
        </CardHeader>
        <CardContent>
          {filteredLeaderboard.length === 0 ? (
            <div className="text-center py-8 text-text-secondary">
              <Search className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No users found</p>
              <p className="text-sm">Try adjusting your search terms</p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredLeaderboard.map((user, index) => (
                <div
                  key={user.id}
                  className={`flex items-center space-x-4 p-4 rounded-lg border transition-colors hover:bg-surface/50 ${
                    user.username === user.username ? 'bg-primary/5 border-primary/20' : 'border-surface'
                  }`}
                >
                  {/* Rank */}
                  <div className="flex items-center justify-center w-10">
                    {getRankIcon(index + 1)}
                  </div>
                  
                  {/* Avatar and Info */}
                  <Avatar className="h-12 w-12">
                    <AvatarImage src={user.avatar} />
                    <AvatarFallback>
                      {getInitials(user.username)}
                    </AvatarFallback>
                  </Avatar>
                  
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center space-x-2">
                      <h4 className="font-medium truncate">{user.username}</h4>
                      {user.username === user.username && (
                        <Badge variant="secondary" className="text-xs">You</Badge>
                      )}
                    </div>
                    <div className="flex items-center space-x-4 text-sm text-text-secondary">
                      <div className="flex items-center space-x-1">
                        <Award className="h-3 w-3" />
                        <span>{user.badgeCount}</span>
                      </div>
                      <div className="flex items-center space-x-1">
                        <TrendingUp className="h-3 w-3" />
                        <span>{user.trustScore}</span>
                      </div>
                      <span>{user.totalShared} files</span>
                    </div>
                  </div>
                  
                  {/* Stats */}
                  <div className="text-right">
                    <div className="text-sm font-medium">{user.badgeCount} badges</div>
                    <div className={`text-xs ${getTrustScoreColor(user.trustScore)}`}>
                      Trust: {user.trustScore}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Achievement Categories */}
      <Card>
        <CardHeader>
          <CardTitle>Achievement Categories</CardTitle>
          <CardDescription>
            Different ways to earn recognition on the network
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <div className="p-4 border border-surface rounded-lg">
              <div className="flex items-center space-x-2 mb-2">
                <Award className="h-5 w-5 text-warning" />
                <h4 className="font-medium">Community Helper</h4>
              </div>
              <p className="text-sm text-text-secondary">
                Help other users and maintain high trust scores
              </p>
            </div>
            
            <div className="p-4 border border-surface rounded-lg">
              <div className="flex items-center space-x-2 mb-2">
                <TrendingUp className="h-5 w-5 text-success" />
                <h4 className="font-medium">Speed Demon</h4>
              </div>
              <p className="text-sm text-text-secondary">
                Maintain fast upload and download speeds
              </p>
            </div>
            
            <div className="p-4 border border-surface rounded-lg">
              <div className="flex items-center space-x-2 mb-2">
                <Trophy className="h-5 w-5 text-primary" />
                <h4 className="font-medium">File Wizard</h4>
              </div>
              <p className="text-sm text-text-secondary">
                Share rare and popular files with the community
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
