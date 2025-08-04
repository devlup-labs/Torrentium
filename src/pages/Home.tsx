import { Upload, Download, Network, TrendingUp } from 'lucide-react'

export default function Home() {
  return (
    <div className="space-y-8">
      {/* Welcome Section */}
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold text-foreground">
          Welcome to Torrentium
        </h1>
        <p className="text-lg text-muted-foreground max-w-2xl mx-auto">
          Direct peer-to-peer file sharing that works through firewalls. 
          Experience secure, fast, and decentralized file transfers.
        </p>
      </div>

      {/* Quick Actions */}
      <div className="grid md:grid-cols-2 gap-6">
        {/* Upload Section */}
        <div className="bg-card border border-border rounded-lg p-6 hover:bg-muted/50 transition-colors">
          <div className="flex items-center gap-3 mb-4">
            <Upload className="h-6 w-6 text-primary" />
            <h2 className="text-xl font-semibold text-foreground">Upload & Share</h2>
          </div>
          <p className="text-muted-foreground mb-4">
            Share your files securely with peers around the world
          </p>
          <button className="w-full bg-primary text-primary-foreground py-2 rounded-lg hover:bg-primary/90 transition-colors">
            Start Uploading
          </button>
        </div>

        {/* Download Section */}
        <div className="bg-card border border-border rounded-lg p-6 hover:bg-muted/50 transition-colors">
          <div className="flex items-center gap-3 mb-4">
            <Download className="h-6 w-6 text-primary" />
            <h2 className="text-xl font-semibold text-foreground">Download Files</h2>
          </div>
          <p className="text-muted-foreground mb-4">
            Find and download files from trusted peers
          </p>
          <button className="w-full bg-secondary text-secondary-foreground py-2 rounded-lg hover:bg-secondary/90 transition-colors">
            Browse Files
          </button>
        </div>
      </div>

      {/* Network Stats */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-xl font-semibold text-foreground mb-6 flex items-center gap-3">
          <Network className="h-6 w-6 text-primary" />
          Network Statistics
        </h2>
        
        <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
          <div className="text-center">
            <div className="text-2xl font-bold text-primary">1,247</div>
            <div className="text-sm text-muted-foreground">Active Peers</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-primary">89.2%</div>
            <div className="text-sm text-muted-foreground">Network Health</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-primary">42.8 TB</div>
            <div className="text-sm text-muted-foreground">Data Shared</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-primary">156K</div>
            <div className="text-sm text-muted-foreground">Files Available</div>
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-xl font-semibold text-foreground mb-6 flex items-center gap-3">
          <TrendingUp className="h-6 w-6 text-primary" />
          Recent Activity
        </h2>
        
        <div className="space-y-3">
          {[
            { action: 'Downloaded', file: 'ubuntu-22.04.3-desktop-amd64.iso', time: '2 minutes ago' },
            { action: 'Uploaded', file: 'presentation-slides.pdf', time: '15 minutes ago' },
            { action: 'Shared', file: 'nature-documentary-4k.mkv', time: '1 hour ago' },
          ].map((activity, index) => (
            <div key={index} className="flex items-center justify-between py-2 border-b border-border last:border-b-0">
              <div>
                <span className="text-foreground font-medium">{activity.action}</span>
                <span className="text-muted-foreground ml-2">{activity.file}</span>
              </div>
              <span className="text-sm text-muted-foreground">{activity.time}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}