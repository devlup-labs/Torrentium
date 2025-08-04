import { useState } from 'react'
import { User } from 'lucide-react'
import Sidebar from './Sidebar'
import TopNavigation from './TopNavigation'
import ProfileModal from './ProfileModal'

interface LayoutProps {
  children: React.ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const [isProfileOpen, setIsProfileOpen] = useState(false)

  return (
    <div className="min-h-screen flex w-full bg-background">
      {/* Auto-hiding Sidebar */}
      <Sidebar />
      
      {/* Main Content Area */}
      <div className="flex-1 flex flex-col">
        {/* Top Navigation with Profile Icon */}
        <header className="h-16 bg-card border-b border-border flex items-center justify-between px-6">
          <TopNavigation />
          
          {/* Profile Icon */}
          <button
            onClick={() => setIsProfileOpen(true)}
            className="p-2 rounded-lg hover:bg-muted transition-colors"
            aria-label="Open Profile"
          >
            <User className="h-5 w-5 text-muted-foreground hover:text-foreground" />
          </button>
        </header>

        {/* Main Content */}
        <main className="flex-1 p-6">
          {children}
        </main>
      </div>

      {/* Profile Modal */}
      <ProfileModal 
        isOpen={isProfileOpen} 
        onClose={() => setIsProfileOpen(false)} 
      />
    </div>
  )
}