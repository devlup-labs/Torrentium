import React from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { 
  Monitor, 
  Volume2, 
  Bell, 
  Shield, 
  Download, 
  Upload, 
  HardDrive,
  Network,
  Eye,
  Moon,
  Sun,
  Wifi,
  Database
} from 'lucide-react';

export default function Settings() {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-text mb-2">Settings</h1>
        <p className="text-text-secondary">Customize your Torrentium experience</p>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* General Settings */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <Monitor className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">General</h2>
          </div>
          
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text mb-2">Theme</label>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" className="flex items-center gap-2">
                  <Moon className="h-4 w-4" />
                  Dark
                </Button>
                <Button variant="secondary" size="sm" className="flex items-center gap-2">
                  <Sun className="h-4 w-4" />
                  Light
                </Button>
              </div>
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Language</label>
              <select className="w-full px-3 py-2 bg-background border border-surface rounded-lg text-text focus:outline-none focus:ring-2 focus:ring-primary">
                <option>English</option>
                <option>Spanish</option>
                <option>French</option>
                <option>German</option>
              </select>
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Auto-start with system</label>
              <div className="flex items-center gap-2">
                <input type="checkbox" className="rounded border-surface" defaultChecked />
                <span className="text-sm text-text-secondary">Launch Torrentium when system starts</span>
              </div>
            </div>
          </div>
        </Card>

        {/* Notifications */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <Bell className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">Notifications</h2>
          </div>
          
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-text">Download completed</p>
                <p className="text-xs text-text-secondary">Show notification when downloads finish</p>
              </div>
              <input type="checkbox" className="rounded border-surface" defaultChecked />
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-text">Upload completed</p>
                <p className="text-xs text-text-secondary">Notify when uploads are finished</p>
              </div>
              <input type="checkbox" className="rounded border-surface" defaultChecked />
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-text">Peer connected</p>
                <p className="text-xs text-text-secondary">Alert when new peers join</p>
              </div>
              <input type="checkbox" className="rounded border-surface" />
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium text-text">Sound notifications</p>
                <p className="text-xs text-text-secondary">Play sound effects</p>
              </div>
              <input type="checkbox" className="rounded border-surface" defaultChecked />
            </div>
          </div>
        </Card>

        {/* Network Settings */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <Network className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">Network</h2>
          </div>
          
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text mb-2">Listening Port</label>
              <Input type="number" defaultValue="6881" placeholder="6881" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Max Connections</label>
              <Input type="number" defaultValue="200" placeholder="200" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Upload Limit (KB/s)</label>
              <Input type="number" defaultValue="0" placeholder="0 (unlimited)" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Download Limit (KB/s)</label>
              <Input type="number" defaultValue="0" placeholder="0 (unlimited)" />
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Enable DHT (Distributed Hash Table)</span>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Enable PEX (Peer Exchange)</span>
            </div>
          </div>
        </Card>

        {/* Privacy & Security */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <Shield className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">Privacy & Security</h2>
          </div>
          
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Enable encryption</span>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" />
              <span className="text-sm text-text-secondary">Use proxy for connections</span>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Anonymous mode</span>
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">User Agent</label>
              <Input defaultValue="Torrentium/1.0" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Peer ID Prefix</label>
              <Input defaultValue="-TR1000-" />
            </div>
          </div>
        </Card>

        {/* Storage Settings */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <HardDrive className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">Storage</h2>
          </div>
          
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text mb-2">Default Download Location</label>
              <div className="flex gap-2">
                <Input defaultValue="C:/Users/Downloads/Torrentium" className="flex-1" />
                <Button variant="outline" size="sm">Browse</Button>
              </div>
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Completed Downloads</label>
              <div className="flex gap-2">
                <Input defaultValue="C:/Users/Downloads/Completed" className="flex-1" />
                <Button variant="outline" size="sm">Browse</Button>
              </div>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Move completed downloads automatically</span>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" />
              <span className="text-sm text-text-secondary">Pre-allocate disk space</span>
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Cache Size (MB)</label>
              <Input type="number" defaultValue="64" />
            </div>
          </div>
        </Card>

        {/* Advanced Settings */}
        <Card className="p-6">
          <div className="flex items-center gap-3 mb-6">
            <Database className="h-5 w-5 text-primary" />
            <h2 className="text-lg font-semibold text-text">Advanced</h2>
          </div>
          
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-text mb-2">Tracker Announce Interval</label>
              <Input type="number" defaultValue="1800" placeholder="1800 seconds" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Max Active Downloads</label>
              <Input type="number" defaultValue="5" />
            </div>
            
            <div>
              <label className="block text-sm font-medium text-text mb-2">Max Active Uploads</label>
              <Input type="number" defaultValue="3" />
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" />
              <span className="text-sm text-text-secondary">Enable µTP (Micro Transport Protocol)</span>
            </div>
            
            <div className="flex items-center gap-2">
              <input type="checkbox" className="rounded border-surface" defaultChecked />
              <span className="text-sm text-text-secondary">Auto-manage torrents</span>
            </div>
            
            <div className="pt-4 border-t border-surface">
              <Button variant="destructive" size="sm">Reset to Defaults</Button>
            </div>
          </div>
        </Card>
      </div>

      {/* Save Button */}
      <div className="flex justify-end gap-3">
        <Button variant="outline">Cancel</Button>
        <Button>Save Changes</Button>
      </div>
    </div>
  );
}
