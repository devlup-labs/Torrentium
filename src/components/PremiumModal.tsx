import { X, Check, Crown } from 'lucide-react'

interface PremiumModalProps {
  isOpen: boolean
  onClose: () => void
}

export default function PremiumModal({ isOpen, onClose }: PremiumModalProps) {
  if (!isOpen) return null

  const freeFeatures = [
    'Trust-based peer selection and scoring',
    'Basic network security',
    'Limited personal torrent networks',
    'Cap on nodes per network',
    'Ads based on usage'
  ]

  const premiumFeatures = [
    'Higher caps on networks and nodes',
    'Disable ads',
    'Plugin system access',
    'Custom trust thresholds',
    'Advanced seeding behavior controls',
    'Peer discovery priority'
  ]

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-card border border-border rounded-lg p-6 w-full max-w-4xl mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Crown className="h-6 w-6 text-yellow-500" />
            <h2 className="text-2xl font-bold text-foreground">Upgrade to Premium</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-muted rounded-lg transition-colors"
          >
            <X className="h-5 w-5 text-muted-foreground" />
          </button>
        </div>

        {/* Feature Comparison */}
        <div className="grid md:grid-cols-2 gap-6">
          {/* Free Version */}
          <div className="space-y-4">
            <div className="text-center">
              <h3 className="text-xl font-semibold text-foreground mb-2">Free Version</h3>
              <div className="text-3xl font-bold text-muted-foreground">$0</div>
              <div className="text-sm text-muted-foreground">Forever</div>
            </div>
            
            <ul className="space-y-3">
              {freeFeatures.map((feature, index) => (
                <li key={index} className="flex items-start gap-3">
                  <Check className="h-5 w-5 text-green-500 flex-shrink-0 mt-0.5" />
                  <span className="text-sm text-muted-foreground">{feature}</span>
                </li>
              ))}
            </ul>
          </div>

          {/* Premium Version */}
          <div className="space-y-4 relative">
            <div className="absolute -top-3 -right-3 bg-gradient-to-r from-yellow-500 to-orange-500 text-white px-3 py-1 rounded-full text-xs font-medium">
              RECOMMENDED
            </div>
            
            <div className="text-center">
              <h3 className="text-xl font-semibold text-foreground mb-2">Premium</h3>
              <div className="text-3xl font-bold text-primary">$9.99</div>
              <div className="text-sm text-muted-foreground">Per month</div>
            </div>
            
            <ul className="space-y-3">
              {premiumFeatures.map((feature, index) => (
                <li key={index} className="flex items-start gap-3">
                  <Check className="h-5 w-5 text-primary flex-shrink-0 mt-0.5" />
                  <span className="text-sm text-foreground">{feature}</span>
                </li>
              ))}
            </ul>

            <button className="w-full bg-gradient-to-r from-yellow-500 to-orange-500 text-white py-3 rounded-lg font-medium hover:from-yellow-600 hover:to-orange-600 transition-all duration-200 shadow-lg hover:shadow-xl">
              Upgrade Now
            </button>
          </div>
        </div>

        {/* Benefits Section */}
        <div className="mt-8 pt-6 border-t border-border">
          <h4 className="text-lg font-semibold text-foreground mb-4">Why Upgrade?</h4>
          <div className="grid md:grid-cols-3 gap-4">
            <div className="text-center p-4 bg-muted rounded-lg">
              <Crown className="h-8 w-8 text-yellow-500 mx-auto mb-2" />
              <h5 className="font-medium text-foreground mb-1">Priority Access</h5>
              <p className="text-xs text-muted-foreground">Get priority in peer discovery and faster connections</p>
            </div>
            <div className="text-center p-4 bg-muted rounded-lg">
              <Check className="h-8 w-8 text-green-500 mx-auto mb-2" />
              <h5 className="font-medium text-foreground mb-1">Ad-Free Experience</h5>
              <p className="text-xs text-muted-foreground">Enjoy uninterrupted torrenting without any ads</p>
            </div>
            <div className="text-center p-4 bg-muted rounded-lg">
              <Crown className="h-8 w-8 text-primary mx-auto mb-2" />
              <h5 className="font-medium text-foreground mb-1">Advanced Controls</h5>
              <p className="text-xs text-muted-foreground">Fine-tune your torrenting with advanced settings</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}