import React, { useState, useEffect } from 'react'
import { NavLink } from 'react-router-dom'
import { 
  House, 
  CloudUpload, 
  CloudDownload, 
  Clock, 
  Shield, 
  UserCircle, 
  Trophy, 
  Menu,
  X,
  Settings,
  Sparkles
} from 'lucide-react'
import { useApp } from '../context/AppContext'
import { cn } from '../lib/utils'
import { Button } from './ui/Button'

const navigationItems = [
  { to: '/', icon: House, label: 'Home' },
  { to: '/upload', icon: CloudUpload, label: 'Upload' },
  { to: '/download', icon: CloudDownload, label: 'Download' },
  { to: '/history', icon: Clock, label: 'History' },
  { to: '/network', icon: Shield, label: 'Private Network' },
  { to: '/profile', icon: UserCircle, label: 'Profile' },
  { to: '/leaderboard', icon: Trophy, label: 'Leaderboard' },
]

const bottomItems = [
  { to: '/settings', icon: Settings, label: 'Settings' },
  { to: '/premium', icon: Sparkles, label: 'Premium' },
]

export function Sidebar() {
  const { dispatch } = useApp()
  const [isHovered, setIsHovered] = useState(false)
  const [isMobileOpen, setIsMobileOpen] = useState(false)
  const [hoverTimeout, setHoverTimeout] = useState(null)

  const toggleMobileSidebar = () => {
    setIsMobileOpen(!isMobileOpen)
  }

  const handleMouseEnter = () => {
    if (hoverTimeout) {
      clearTimeout(hoverTimeout)
    }
    setIsHovered(true)
  }

  const handleMouseLeave = () => {
    const timeout = setTimeout(() => {
      setIsHovered(false)
    }, 100) // Small delay to prevent jittery behavior
    setHoverTimeout(timeout)
  }

  const isExpanded = isHovered || isMobileOpen

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (hoverTimeout) {
        clearTimeout(hoverTimeout)
      }
    }
  }, [hoverTimeout])

  return (
    <>
      {/* Mobile backdrop */}
      {isMobileOpen && (
        <div 
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={toggleMobileSidebar}
        />
      )}
      
      {/* Mobile hamburger button */}
      <Button
        variant="ghost"
        size="icon"
        onClick={toggleMobileSidebar}
        className="fixed top-4 left-4 z-50 md:hidden bg-surface border border-background"
      >
        {isMobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
      </Button>
      
      {/* Sidebar */}
      <div 
        className={cn(
          "fixed left-0 top-0 h-full bg-surface border-r border-background z-40 transition-all duration-300 ease-out transform-gpu",
          "flex flex-col group overflow-hidden",
          // Mobile behavior
          isMobileOpen ? "w-64 translate-x-0" : "-translate-x-full",
          // Desktop behavior
          "md:translate-x-0 md:w-16 md:hover:w-64"
        )}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        {/* Header */}
        <div className="flex items-center p-3 border-b border-background min-h-[65px] flex-shrink-0">
          <div className={cn(
            "flex items-center transition-all duration-300 ease-out transform-gpu",
            isExpanded ? "space-x-3" : "justify-center w-full"
          )}>
            <div className="w-7 h-7 bg-primary rounded-lg flex items-center justify-center flex-shrink-0">
              <span className="text-background font-bold text-xs">T</span>
            </div>
            {isExpanded && (
              <span className="font-semibold text-base whitespace-nowrap">
                Torrentium
              </span>
            )}
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-3 overflow-y-auto">
          <ul className="space-y-2">
            {navigationItems.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  className={({ isActive }) => cn(
                    "flex items-center h-10 transition-all duration-300 ease-out group relative transform-gpu rounded-lg",
                    isActive 
                      ? "text-primary bg-primary/10" 
                      : "text-text-secondary hover:bg-background/40 hover:text-text",
                    isExpanded ? "px-3 justify-start" : "justify-center w-10 mx-auto"
                  )}
                >
                  <div className="flex items-center justify-center w-5 h-5 flex-shrink-0">
                    <item.icon className="w-full h-full" />
                  </div>
                  {isExpanded && (
                    <span className="ml-3 font-medium whitespace-nowrap">
                      {item.label}
                    </span>
                  )}
                  {/* Tooltip for collapsed state */}
                  {!isExpanded && (
                    <div className="absolute left-full ml-2 px-2 py-1 bg-surface/95 backdrop-blur-sm border border-background rounded text-xs font-medium shadow-lg z-50 whitespace-nowrap transition-all duration-150 ease-out opacity-0 scale-95 translate-x-1 pointer-events-none group-hover:opacity-100 group-hover:scale-100 group-hover:translate-x-0 group-hover:delay-500">
                      {item.label}
                      <div className="absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1 w-1 h-1 bg-surface/95 border-l border-b border-background rotate-45"></div>
                    </div>
                  )}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        {/* Bottom Navigation */}
        <div className="p-3 border-t border-background flex-shrink-0">
          <ul className="space-y-2">
            {bottomItems.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  className={({ isActive }) => cn(
                    "flex items-center h-10 transition-all duration-300 ease-out group relative transform-gpu rounded-lg",
                    isActive 
                      ? "text-primary bg-primary/10" 
                      : "text-text-secondary hover:bg-background/40 hover:text-text",
                    isExpanded ? "px-3 justify-start" : "justify-center w-10 mx-auto"
                  )}
                >
                  <div className="flex items-center justify-center w-5 h-5 flex-shrink-0">
                    <item.icon className="w-full h-full" />
                  </div>
                  {isExpanded && (
                    <span className="ml-3 font-medium whitespace-nowrap">
                      {item.label}
                    </span>
                  )}
                  {/* Tooltip for collapsed state */}
                  {!isExpanded && (
                    <div className="absolute left-full ml-2 px-2 py-1 bg-surface/95 backdrop-blur-sm border border-background rounded text-xs font-medium shadow-lg z-50 whitespace-nowrap transition-all duration-150 ease-out opacity-0 scale-95 translate-x-1 pointer-events-none group-hover:opacity-100 group-hover:scale-100 group-hover:translate-x-0 group-hover:delay-500">
                      {item.label}
                      <div className="absolute left-0 top-1/2 -translate-y-1/2 -translate-x-1 w-1 h-1 bg-surface/95 border-l border-b border-background rotate-45"></div>
                    </div>
                  )}
                </NavLink>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </>
  )
}
