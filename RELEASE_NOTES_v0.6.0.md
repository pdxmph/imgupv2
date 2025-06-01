# imgupv2 v0.6.0 Release Notes

## 🎉 Major Feature: Bluesky Integration

imgupv2 now supports posting to Bluesky! Share your photos across Flickr, Mastodon, and Bluesky with a single command.

### ✨ New Features

#### Bluesky Support
- **Authentication**: Simple app password authentication with `imgup auth bluesky`
- **Image Posting**: Share photos to Bluesky with automatic alt text support
- **Crossposting**: Use `--mastodon --bluesky` to post to both platforms simultaneously
- **Smart URL Detection**: All URLs in posts are automatically made clickable
- **GUI Integration**: New Bluesky checkbox with synchronized post text fields

#### Safety & Testing Features
- **`--dry-run` Flag**: Preview what will be posted without actually posting
- **Visibility Warnings**: Clear warnings when crossposting with private visibility (Bluesky posts are always public)
- **Character Limit Handling**: Automatic warnings for Bluesky's 300 character limit

### 🛠 Technical Details

- Bluesky API client with proper AT Protocol support
- Automatic URL facet detection for clickable links
- Shared `--post` flag for seamless crossposting
- Full integration in both CLI and GUI

### 📝 Usage Examples

```bash
# Authenticate with Bluesky
imgup auth bluesky

# Post to Bluesky
imgup upload photo.jpg --bluesky --post "Check out this shot!" --alt "Mountain sunset"

# Crosspost to Mastodon and Bluesky
imgup upload photo.jpg --mastodon --bluesky --post "New photo!" --visibility direct

# Test without posting
imgup upload photo.jpg --bluesky --post "Test post" --dry-run
```

### 🔧 Configuration

Set up Bluesky in your config:
```bash
imgup config set bluesky.handle yourhandle.bsky.social
imgup config set bluesky.app_password xxxx-xxxx-xxxx-xxxx
```

### ⚠️ Important Notes

- All Bluesky posts are public (no privacy settings)
- 300 character limit for Bluesky posts
- Create app passwords at https://bsky.app/settings/app-passwords

### 🐛 Bug Fixes

- Updated Homebrew cask for proper v0.5.0 installation

---

**Full Changelog**: https://github.com/pdxmph/imgupv2/compare/v0.5.0...v0.6.0
