import { User, Settings, Shield, Download, Upload } from 'lucide-react'

export default function Profile() {
  return (
    <div className="max-w-4xl mx-auto space-y-8">
      {/* Profile Header */}
      <div className="bg-card border border-border rounded-lg p-6">
        <div className="flex items-start gap-6">
          {/* Profile Picture */}
          <div className="w-24 h-24 bg-muted rounded-full flex items-center justify-center">
            <User className="h-12 w-12 text-muted-foreground" />
          </div>
          
          {/* Profile Info */}
          <div className="flex-1">
            <h1 className="text-2xl font-bold text-foreground mb-2">TorrentUser123</h1>
            <p className="text-muted-foreground mb-4">Member since January 2024</p>
            
            {/* Trust Score */}
            <div className="space-y-2">
              <div className="flex justify-between items-center">
                <span className="text-sm font-medium text-foreground">Trust Score</span>
                <span className="text-lg font-bold text-primary">0.8 / 1.0</span>
              </div>
              <div className="w-full bg-muted rounded-full h-3">
                <div 
                  className="bg-primary h-3 rounded-full transition-all duration-300"
                  style={{ width: '80%' }}
                />
              </div>
              <p className="text-sm text-muted-foreground">
                Excellent trust rating based on your sharing history and community feedback
              </p>
            </div>
          </div>

          {/* Settings Button */}
          <button className="p-2 hover:bg-muted rounded-lg transition-colors">
            <Settings className="h-5 w-5 text-muted-foreground" />
          </button>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid md:grid-cols-3 gap-6">
        {/* Upload Stats */}
        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <Upload className="h-6 w-6 text-green-500" />
            <h2 className="text-lg font-semibold text-foreground">Uploads</h2>
          </div>
          <div className="space-y-2">
            <div className="text-2xl font-bold text-primary">47</div>
            <div className="text-sm text-muted-foreground">Files Shared</div>
            <div className="text-sm text-muted-foreground">2.3 TB Total</div>
          </div>
        </div>

        {/* Download Stats */}
        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <Download className="h-6 w-6 text-blue-500" />
            <h2 className="text-lg font-semibold text-foreground">Downloads</h2>
          </div>
          <div className="space-y-2">
            <div className="text-2xl font-bold text-primary">156</div>
            <div className="text-sm text-muted-foreground">Files Downloaded</div>
            <div className="text-sm text-muted-foreground">5.7 TB Total</div>
          </div>
        </div>

        {/* Security Stats */}
        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <Shield className="h-6 w-6 text-purple-500" />
            <h2 className="text-lg font-semibold text-foreground">Security</h2>
          </div>
          <div className="space-y-2">
            <div className="text-2xl font-bold text-primary">100%</div>
            <div className="text-sm text-muted-foreground">Verified Files</div>
            <div className="text-sm text-muted-foreground">No Issues</div>
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-xl font-semibold text-foreground mb-6">Recent Activity</h2>
        
        <div className="space-y-4">
          {[
            { 
              type: 'download', 
              file: 'ubuntu-22.04.3-desktop-amd64.iso', 
              size: '4.6 GB',
              time: '2 minutes ago',
              status: 'completed'
            },
            { 
              type: 'upload', 
              file: 'presentation-slides.pdf', 
              size: '12.4 MB',
              time: '15 minutes ago',
              status: 'sharing'
            },
            { 
              type: 'download', 
              file: 'nature-documentary-4k.mkv', 
              size: '8.2 GB',
              time: '1 hour ago',
              status: 'completed'
            },
            { 
              type: 'upload', 
              file: 'open-source-project.zip', 
              size: '156 MB',
              time: '3 hours ago',
              status: 'sharing'
            },
          ].map((activity, index) => (
            <div key={index} className="flex items-center justify-between py-3 border-b border-border last:border-b-0">
              <div className="flex items-center gap-3">
                {activity.type === 'download' ? (
                  <Download className="h-4 w-4 text-blue-500" />
                ) : (
                  <Upload className="h-4 w-4 text-green-500" />
                )}
                <div>
                  <div className="text-foreground font-medium">{activity.file}</div>
                  <div className="text-sm text-muted-foreground">{activity.size}</div>
                </div>
              </div>
              <div className="text-right">
                <div className={`text-sm font-medium ${
                  activity.status === 'completed' ? 'text-green-500' : 'text-blue-500'
                }`}>
                  {activity.status === 'completed' ? 'Completed' : 'Sharing'}
                </div>
                <div className="text-sm text-muted-foreground">{activity.time}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Network Connections */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-xl font-semibold text-foreground mb-6">Network Connections</h2>
        
        <div className="space-y-3">
          <div className="flex justify-between items-center">
            <span className="text-foreground">Active Connections</span>
            <span className="text-primary font-medium">12 peers</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-foreground">Upload Speed</span>
            <span className="text-green-500 font-medium">2.4 MB/s</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-foreground">Download Speed</span>
            <span className="text-blue-500 font-medium">8.7 MB/s</span>
          </div>
          <div className="flex justify-between items-center">
            <span className="text-foreground">Network Status</span>
            <span className="text-green-500 font-medium">Connected</span>
          </div>
        </div>
      </div>
    </div>
  )
}