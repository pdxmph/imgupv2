# imgupv2 Duplicate Detection & Cache

## How It Works

imgupv2 uses a local SQLite cache to track uploaded photos and avoid duplicates:

- **Cache location**: `~/.config/imgupv2/uploads.db`
- **Tracks**: MD5 hash, photo ID, URLs, upload time
- **Enabled by default** (can be disabled in config)

## Important Notes

### Deleted Photos

If you delete photos from Flickr/SmugMug, the cache will still contain references to them. This can cause issues when:
- Using the cached URLs for social media posting
- The GUI showing photos as "duplicates" when they're actually deleted

### Solutions

1. **Clear the cache** when you've deleted photos:
   ```bash
   rm ~/.config/imgupv2/uploads.db
   ```

2. **Force upload** to bypass duplicate detection:
   ```bash
   imgup upload photo.jpg --force
   ```

3. **Disable cache entirely** if it's too much overhead:
   ```bash
   imgup config set default.duplicate_check false
   ```

## Philosophy

imgupv2 is designed as a "fire and forget" uploader. The duplicate detection is a convenience feature, not a photo management system. If the cache causes issues, just clear it or disable it.

## Trade-offs

- **With cache**: Fast duplicate detection, but requires manual clearing after deletions
- **Without cache**: No duplicate detection, but no maintenance needed
- **Force flag**: Bypasses cache for one-off uploads

Choose what works best for your workflow.
