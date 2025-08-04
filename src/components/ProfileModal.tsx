import { X, User } from 'lucide-react'

interface ProfileModalProps {
  isOpen: boolean
  onClose: () => void
}

export default function ProfileModal({ isOpen, onClose }: ProfileModalProps) {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-card border border-border rounded-lg p-6 w-full max-w-md mx-4">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold text-foreground">My Profile</h2>
          <button
            onClick={onClose}
            className="p-1 hover:bg-muted rounded-lg transition-colors"
          >
            <X className="h-5 w-5 text-muted-foreground" />
          </button>
        </div>

        {/* Profile Content */}
        <div className="space-y-6">
          {/* Profile Picture */}
          <div className="flex justify-center">
            <div className="w-24 h-24 bg-muted rounded-full flex items-center justify-center">
              <User className="h-12 w-12 text-muted-foreground" />
            </div>
          </div>

          {/* Username */}
          <div className="text-center">
            <h3 className="text-lg font-medium text-foreground">TorrentUser123</h3>
            <p className="text-sm text-muted-foreground">Member since 2024</p>
          </div>

          {/* Trust Score */}
          <div className="space-y-2">
            <div className="flex justify-between items-center">
              <span className="text-sm font-medium text-foreground">Trust Score</span>
              <span className="text-sm font-bold text-primary">0.8</span>
            </div>
            <div className="w-full bg-muted rounded-full h-2">
              <div 
                className="bg-primary h-2 rounded-full transition-all duration-300"
                style={{ width: '80%' }}
              />
            </div>
            <p className="text-xs text-muted-foreground">
              High trust rating based on your sharing history
            </p>
          </div>

          {/* Profile Stats */}
          <div className="grid grid-cols-2 gap-4 pt-4 border-t border-border">
            <div className="text-center">
              <div className="text-2xl font-bold text-primary">47</div>
              <div className="text-xs text-muted-foreground">Files Shared</div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-primary">156</div>
              <div className="text-xs text-muted-foreground">Downloads</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}