import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { Home, User, Crown } from 'lucide-react'
import PremiumModal from './PremiumModal'

export default function Sidebar() {
  const [isExpanded, setIsExpanded] = useState(false)
  const [isPremiumOpen, setIsPremiumOpen] = useState(false)

  const navigation = [
    { name: 'Home', href: '/', icon: Home },
    { name: 'Profile', href: '/profile', icon: User },
  ]

  return (
    <>
      <div 
        className={`bg-card border-r border-border transition-all duration-300 flex flex-col h-screen group ${
          isExpanded ? 'w-64' : 'w-16'
        }`}
        onMouseEnter={() => setIsExpanded(true)}
        onMouseLeave={() => setIsExpanded(false)}
      >
        {/* Navigation Items */}
        <nav className="flex-1 p-4 space-y-2">
          {navigation.map((item) => (
            <NavLink
              key={item.name}
              to={item.href}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-3 rounded-lg transition-colors ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                }`
              }
            >
              <item.icon className="h-5 w-5 flex-shrink-0" />
              <span className={`transition-opacity duration-200 ${
                isExpanded ? 'opacity-100' : 'opacity-0'
              }`}>
                {item.name}
              </span>
            </NavLink>
          ))}
        </nav>

        {/* Upgrade to Premium Button - Bottom */}
        <div className="p-4">
          <button
            onClick={() => setIsPremiumOpen(true)}
            className={`w-full flex items-center justify-center gap-3 px-4 py-4 bg-gradient-to-r from-yellow-500 to-orange-500 text-white rounded-lg hover:from-yellow-600 hover:to-orange-600 transition-all duration-200 shadow-lg hover:shadow-xl ${
              isExpanded ? 'text-sm font-medium' : 'p-3'
            }`}
          >
            <Crown className="h-5 w-5 flex-shrink-0" />
            <span className={`transition-opacity duration-200 ${
              isExpanded ? 'opacity-100' : 'opacity-0'
            }`}>
              Upgrade to Premium
            </span>
          </button>
        </div>
      </div>

      <PremiumModal 
        isOpen={isPremiumOpen} 
        onClose={() => setIsPremiumOpen(false)} 
      />
    </>
  )
}