import React from 'react'
import { Outlet, Link } from 'react-router-dom'
import { User, Award } from 'lucide-react'
import { Sidebar } from './Sidebar'
import { ToastContainer } from './Toast'
import { Button } from './ui/Button'

export function Layout() {
  return (
    <div className="min-h-screen bg-background text-text">
      <Sidebar />
      {/* Top Header */}
      <header className="fixed top-0 right-0 z-30 p-4 md:ml-16">
        <div className="flex items-center space-x-2">
          <Button variant="ghost" size="sm" asChild>
            <Link to="/profile">
              <User className="h-5 w-5" />
            </Link>
          </Button>
          <Button variant="ghost" size="sm" asChild>
            <Link to="/leaderboard">
              <Award className="h-5 w-5" />
            </Link>
          </Button>
        </div>
      </header>
      <main className="p-6 pt-16 md:ml-16 transition-all duration-300">
        <Outlet />
      </main>
      <ToastContainer />
    </div>
  )
}
