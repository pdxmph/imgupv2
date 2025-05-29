# Flickr API Setup

To use imgupv2 with Flickr, you need to create a Flickr app to get API credentials.

## Steps:

1. Go to https://www.flickr.com/services/apps/create/

2. Choose "Apply for a Non-Commercial Key"

3. Fill in the form:
   - App Name: Something like "imgupv2" or "Personal Image Uploader"
   - Description: "Personal command-line tool for uploading images"
   - Check the boxes to agree to terms

4. After creation, you'll see:
   - Key: (your consumer key)
   - Secret: (your consumer secret)

5. Configure imgupv2:
   ```bash
   imgup config set flickr.key YOUR_KEY_HERE
   imgup config set flickr.secret YOUR_SECRET_HERE
   ```

6. Authenticate:
   ```bash
   imgup auth flickr
   ```

The auth command will open your browser for authorization. No need to copy/paste URLs!
