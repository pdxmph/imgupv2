// imgupv2 GUI main.js
console.log('main.js loaded');

// Store current photo metadata globally
let currentPhotoMetadata = null;
let currentPhotosArray = null; // For multi-selection

// Track event listeners for cleanup
let thumbnailEventOff = null;
let metadataEventOff = null;

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', async () => {
    console.log('DOM loaded, initializing app...');
    
    // Set up event listeners FIRST before any backend calls
    setupEventListeners();
    
    // Set up tag autocomplete
    setupTagAutocomplete();
    
    // Handle form submission
    document.getElementById('upload-form').onsubmit = handleUpload;
    
    // Handle cancel button
    document.getElementById('cancel-btn').onclick = () => {
        window.runtime.Quit();
    };
    
    // Clean up on window close
    window.addEventListener('beforeunload', () => {
        cleanupMultiPhotoListeners();
    });
    
    // NOW load the selected photo (after event listeners are ready)
    await loadSelectedPhoto();
});

// Set up all event listeners
function setupEventListeners() {
    console.log('Setting up event listeners...');
    
    // Listen for async thumbnail updates
    window.runtime.EventsOn('thumbnail-ready', (data) => {
        console.log('Thumbnail ready event:', data);
        
        // For single photo mode
        if (currentPhotoMetadata && !window.multiPhotoData) {
            // Check if this is for the current photo
            // For Photos.app selections, path might be empty initially
            if ((data.path && data.path === currentPhotoMetadata.path) || 
                (currentPhotoMetadata.isFromPhotos && data.index === 0)) {
                const preview = document.getElementById('preview');
                const container = document.getElementById('preview-container');
                const dimensions = document.getElementById('dimensions');
                
                // Update thumbnail
                if (data.thumbnail) {
                    preview.src = data.thumbnail;
                    preview.style.display = 'block';
                }
                
                // Update dimensions and file size if available
                if (data.width && data.height) {
                    const sizeText = formatFileSize(data.fileSize);
                    dimensions.textContent = `${data.width}×${data.height} • ${sizeText}`;
                } else if (data.path) {
                    // Just show filename for now
                    const filename = document.getElementById('filename');
                    const name = data.path.split('/').pop();
                    filename.textContent = name;
                    dimensions.textContent = '';
                }
                
                // Show container if it was hidden
                container.classList.remove('hidden');
            }
        }
    });
    
    // Listen for Photos export completion
    window.runtime.EventsOn('photos-export-ready', (data) => {
        if (currentPhotoMetadata && currentPhotoMetadata.isFromPhotos) {
            // Update the metadata with the exported path (if provided)
            if (data.path) {
                currentPhotoMetadata.path = data.path;
                currentPhotoMetadata.isTemporary = true;
            }
            
            // Update the preview
            const preview = document.getElementById('preview');
            const dimensions = document.getElementById('dimensions');
            const filename = document.getElementById('filename');
            
            preview.src = data.thumbnail;
            preview.style.display = 'block';
            
            // Update filename to show it's ready
            filename.textContent = data.cached ? 'Photos (Cached)' : 'Photos Export Ready';
            
            if (data.width && data.height) {
                const sizeText = formatFileSize(data.fileSize);
                dimensions.textContent = `${data.width}×${data.height} • ${sizeText}`;
            }
        }
    });
    
    // Listen for Photos path ready (when using cached thumbnail)
    window.runtime.EventsOn('photos-path-ready', (data) => {
        if (currentPhotoMetadata && currentPhotoMetadata.isFromPhotos && data.path) {
            currentPhotoMetadata.path = data.path;
            currentPhotoMetadata.isTemporary = true;
        }
    });
    
    // Listen for async metadata updates
    window.runtime.EventsOn('metadata-ready', (data) => {
        // Update form fields as metadata arrives
        if (data.title && !document.getElementById('title').value) {
            document.getElementById('title').value = data.title;
        }
        if (data.alt && !document.getElementById('alt').value) {
            document.getElementById('alt').value = data.alt;
        }
        if (data.description && !document.getElementById('description').value) {
            document.getElementById('description').value = data.description;
        }
        if (data.tags && data.tags.length > 0 && !document.getElementById('tags').value) {
            document.getElementById('tags').value = data.tags.join(' ');
        }
    });
    
    // Handle Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            window.runtime.Quit();
        }
        
        // Multi-photo keyboard navigation
        if (window.multiPhotoData) {
            const photoCount = window.multiPhotoData.length;
            
            if (e.key === 'Tab' && !e.shiftKey) {
                // Tab to next photo
                e.preventDefault();
                const nextIndex = (window.currentPhotoIndex + 1) % photoCount;
                selectPhoto(nextIndex);
            } else if (e.key === 'Tab' && e.shiftKey) {
                // Shift+Tab to previous photo
                e.preventDefault();
                const prevIndex = (window.currentPhotoIndex - 1 + photoCount) % photoCount;
                selectPhoto(prevIndex);
            } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                // Arrow keys for navigation
                e.preventDefault();
                const delta = e.key === 'ArrowDown' ? 1 : -1;
                const newIndex = Math.max(0, Math.min(photoCount - 1, window.currentPhotoIndex + delta));
                if (newIndex !== window.currentPhotoIndex) {
                    selectPhoto(newIndex);
                }
            }
        }
    });
    
    // Handle Cmd+Enter for quick upload
    document.addEventListener('keydown', (e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
            e.preventDefault();
            document.getElementById('upload-form').dispatchEvent(new Event('submit'));
        }
    });
    
    // Handle social media checkboxes
    const mastodonCheckbox = document.getElementById('mastodon-enabled');
    const blueskyCheckbox = document.getElementById('bluesky-enabled');
    const mastodonOptions = document.getElementById('mastodon-options');
    const blueskyOptions = document.getElementById('bluesky-options');
    const mastodonText = document.getElementById('mastodon-text');
    const blueskyText = document.getElementById('bluesky-text');
    
    // Debug: Track mastodon text changes
    mastodonText.addEventListener('input', (e) => {
        console.log('DEBUG: Mastodon text changed to:', e.target.value);
    });
    
    // Sync post text between services when both are enabled
    function syncPostText(source) {
        const target = source === mastodonText ? blueskyText : mastodonText;
        if (mastodonCheckbox.checked && blueskyCheckbox.checked) {
            target.value = source.value;
        }
    }
    
    mastodonText.addEventListener('input', () => syncPostText(mastodonText));
    blueskyText.addEventListener('input', () => syncPostText(blueskyText));
    
    // Handle Mastodon checkbox
    mastodonCheckbox.addEventListener('change', async (e) => {
        if (e.target.checked) {
            mastodonOptions.classList.remove('hidden');
            // If Bluesky is also checked, sync the text
            if (blueskyCheckbox.checked && blueskyText.value) {
                mastodonText.value = blueskyText.value;
            }
            // Focus on post text if it's empty
            if (!mastodonText.value) {
                mastodonText.focus();
            }
            // Resize window to accommodate extra fields
            try {
                await window.go.main.App.ResizeWindow(true);
            } catch (err) {
                console.error('Failed to resize window:', err);
            }
        } else {
            mastodonOptions.classList.add('hidden');
            // Resize window back if no services are checked
            if (!blueskyCheckbox.checked) {
                try {
                    await window.go.main.App.ResizeWindow(false);
                } catch (err) {
                    console.error('Failed to resize window:', err);
                }
            }
        }
    });
    
    // Handle Bluesky checkbox
    blueskyCheckbox.addEventListener('change', async (e) => {
        if (e.target.checked) {
            blueskyOptions.classList.remove('hidden');
            // If Mastodon is also checked, sync the text
            if (mastodonCheckbox.checked && mastodonText.value) {
                blueskyText.value = mastodonText.value;
            }
            // Focus on post text if it's empty
            if (!blueskyText.value) {
                blueskyText.focus();
            }
            // Resize window to accommodate extra fields
            try {
                await window.go.main.App.ResizeWindow(true);
            } catch (err) {
                console.error('Failed to resize window:', err);
            }
        } else {
            blueskyOptions.classList.add('hidden');
            // Resize window back if no services are checked
            if (!mastodonCheckbox.checked) {
                try {
                    await window.go.main.App.ResizeWindow(false);
                } catch (err) {
                    console.error('Failed to resize window:', err);
                }
            }
        }
    });
    
    console.log('Event listeners setup complete');
}

// Add this function to watch for metadata population
function watchForMetadata() {
    const overlay = document.getElementById('loading-overlay');
    
    // For Finder selections, we're now loading async, so hide overlay immediately
    // The form is already populated with empty fields, and metadata will arrive async
    if (currentPhotoMetadata && currentPhotoMetadata.path && !currentPhotoMetadata.isFromPhotos) {
        overlay.classList.add('fade-out');
        setTimeout(() => {
            overlay.classList.add('hidden');
        }, 300);
        return;
    }
    
    // For Photos selections, also hide immediately since we have the metadata
    if (currentPhotoMetadata && currentPhotoMetadata.isFromPhotos) {
        overlay.classList.add('fade-out');
        setTimeout(() => {
            overlay.classList.add('hidden');
        }, 300);
        return;
    }
    
    // Only use the old watching logic if we somehow don't have metadata yet
    const fieldsToWatch = ['title', 'alt', 'description', 'tags'];
    let hasMetadata = false;
    
    // Check if any fields have content
    const checkFields = () => {
        for (const fieldId of fieldsToWatch) {
            const field = document.getElementById(fieldId);
            if (field && field.value.trim()) {
                hasMetadata = true;
                break;
            }
        }
        
        if (hasMetadata) {
            // Fade out the overlay
            overlay.classList.add('fade-out');
            // Remove after transition
            setTimeout(() => {
                overlay.classList.add('hidden');
            }, 300);
            return true; // Stop checking
        }
        return false;
    };
    
    // Check periodically for metadata
    const checkInterval = setInterval(() => {
        if (checkFields()) {
            clearInterval(checkInterval);
        }
    }, 100);
    
    // Fallback: hide after 1 second regardless (not 5!)
    setTimeout(() => {
        clearInterval(checkInterval);
        overlay.classList.add('fade-out');
        setTimeout(() => {
            overlay.classList.add('hidden');
        }, 300);
    }, 1000);
}

// Extract photo loading logic into a separate function
async function loadSelectedPhoto() {
    const overlay = document.getElementById('loading-overlay');
    
    try {
        // Clear any previous errors
        document.getElementById('error-message').classList.add('hidden');
        document.getElementById('success-message').classList.add('hidden');
        
        // Show the form
        document.getElementById('upload-form').classList.remove('hidden');
        
        // Load selected photos (multiple)
        console.log('Calling GetSelectedPhotos...');
        const photos = await window.go.main.App.GetSelectedPhotos();
        console.log('GetSelectedPhotos returned:', photos);
        
        if (photos && photos.length > 0) {
            currentPhotosArray = photos;
            
            if (photos.length === 1) {
                // Single photo - use existing UI
                currentPhotoMetadata = photos[0];
                populateForm(photos[0]);
                
                // NOW watch for metadata (after we know what type of selection it is)
                watchForMetadata();
                
                // Show a note if this is from Photos
                if (photos[0].isFromPhotos) {
                    showToast('Photo selected from Photos.app');
                }
                
                // Focus on first editable field
                document.getElementById('title').focus();
            } else {
                // Multiple photos - switch to list view
                console.log(`Multiple photos selected: ${photos.length}`);
                showMultiPhotoUI(photos);
            }
        } else {
            // Hide overlay on error
            const overlay = document.getElementById('loading-overlay');
            overlay.classList.add('hidden');
            
            showError('No photo selected in Finder or Photos. Please select a photo and relaunch.');
            // Don't hide the form - let the error overlay show instead
        }
    } catch (err) {
        // Hide overlay on error
        const overlay = document.getElementById('loading-overlay');
        overlay.classList.add('hidden');
        
        console.error('Failed to get selection:', err);
        showError('Failed to get selected photo: ' + err);
        // Don't hide the form - let the error overlay show instead
    } finally {
        // Always hide loading overlay after 2 seconds as a failsafe
        setTimeout(() => {
            const overlay = document.getElementById('loading-overlay');
            if (overlay && !overlay.classList.contains('hidden')) {
                overlay.classList.add('hidden');
                console.error('Loading overlay timeout - forcing hide');
            }
        }, 2000);
    }
}

function populateForm(metadata) {
    // Show single photo preview, hide multi-photo list
    document.getElementById('preview-container').classList.remove('hidden');
    document.getElementById('multi-photo-list').classList.add('hidden');
    
    // Update form fields
    document.getElementById('title').value = metadata.title || '';
    document.getElementById('alt').value = metadata.alt || '';
    document.getElementById('description').value = metadata.description || '';
    document.getElementById('tags').value = (metadata.tags || []).join(' ');
    document.getElementById('format').value = metadata.format || 'markdown';
    document.getElementById('private').checked = metadata.private || false;
    
    // Always call loadPreview to handle all cases (thumbnail, loading, Photos)
    loadPreview(metadata);
    
    // For Photos.app selections without a thumbnail, trigger thumbnail generation
    if (metadata.isFromPhotos && !metadata.thumbnail) {
        console.log('Photos selection missing thumbnail, triggering generation');
        // Small delay to ensure event listeners are fully ready
        setTimeout(() => {
            window.go.main.App.StartThumbnailGeneration([metadata]).then(() => {
                console.log('Thumbnail generation started');
            }).catch(err => {
                console.error('Failed to start thumbnail generation:', err);
            });
        }, 100);
    }
    
    // For Finder selections, also trigger thumbnail and metadata generation
    if (!metadata.isFromPhotos && metadata.path) {
        console.log('Finder selection, triggering thumbnail and metadata generation');
        setTimeout(() => {
            window.go.main.App.StartThumbnailGeneration([metadata]).then(() => {
                console.log('Thumbnail/metadata generation started for Finder file');
            }).catch(err => {
                console.error('Failed to start thumbnail/metadata generation:', err);
            });
        }, 100);
    }
}

function loadPreview(metadata) {
    const preview = document.getElementById('preview');
    const container = document.getElementById('preview-container');
    const filename = document.getElementById('filename');
    const dimensions = document.getElementById('dimensions');
    
    if (metadata.thumbnail) {
        // Use the base64 thumbnail from metadata (cached Photos.app thumbnail)
        preview.src = metadata.thumbnail;
        preview.style.display = 'block';
        
        // Display filename
        if (metadata.photosFilename) {
            filename.textContent = metadata.photosFilename;
        } else if (metadata.path) {
            const name = metadata.path.split('/').pop();
            filename.textContent = name;
        }
        
        // Display dimensions and file size
        if (metadata.imageWidth && metadata.imageHeight) {
            const sizeText = formatFileSize(metadata.fileSize);
            dimensions.textContent = `${metadata.imageWidth}×${metadata.imageHeight} • ${sizeText}`;
        }
        
        container.classList.remove('hidden');
    } else if (metadata.path) {
        // Show placeholder while thumbnail loads
        preview.src = '';
        preview.style.display = 'none';
        
        // Show filename immediately
        const name = metadata.path.split('/').pop();
        filename.textContent = name;
        dimensions.innerHTML = '<span style="color: #666;">Loading preview...</span>';
        
        container.classList.remove('hidden');
        
        // The thumbnail will arrive via the 'thumbnail-ready' event
    } else if (metadata.isFromPhotos) {
        // Photos selection - show spinner while export happens (unless we have cached thumbnail)
        if (!metadata.thumbnail) {
            preview.style.display = 'none';
            filename.textContent = 'Photos Selection';
            dimensions.innerHTML = '<div class="mini-spinner"></div> <span style="color: #666;">Exporting from Photos...</span>';
        } else {
            // We have a cached thumbnail, it was already displayed above
            filename.textContent = metadata.photosFilename || 'Photos Selection';
        }
        container.classList.remove('hidden');
    }
    
    preview.onerror = () => {
        preview.style.display = 'none';
    };
    
    preview.onload = () => {
        preview.style.display = 'block';
    };
}

// Helper to format file size
function formatFileSize(bytes) {
    if (!bytes) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    let size = bytes;
    let unitIndex = 0;
    
    while (size >= 1024 && unitIndex < units.length - 1) {
        size /= 1024;
        unitIndex++;
    }
    
    return `${size.toFixed(1)} ${units[unitIndex]}`;
}

async function setupTagAutocomplete() {
    const tagsInput = document.getElementById('tags');
    const suggestionsDiv = document.getElementById('tag-suggestions');
    
    try {
        const recentTags = await window.go.main.App.GetRecentTags();
        
        tagsInput.addEventListener('input', (e) => {
            const value = e.target.value;
            const words = value.split(' ');
            const currentWord = words[words.length - 1].toLowerCase();
            
            if (currentWord.length < 2) {
                suggestionsDiv.classList.add('hidden');
                return;
            }
            
            const matches = recentTags.filter(tag => 
                tag.toLowerCase().startsWith(currentWord) && 
                !words.slice(0, -1).includes(tag)
            );
            
            if (matches.length > 0) {
                suggestionsDiv.innerHTML = matches
                    .slice(0, 5)
                    .map(tag => `<div class="suggestion-item" data-tag="${tag}">${tag}</div>`)
                    .join('');
                suggestionsDiv.classList.remove('hidden');
                
                // Position suggestions below input
                const rect = tagsInput.getBoundingClientRect();
                suggestionsDiv.style.width = rect.width + 'px';
                suggestionsDiv.style.left = rect.left + 'px';
                suggestionsDiv.style.top = (rect.bottom + window.scrollY) + 'px';
            } else {
                suggestionsDiv.classList.add('hidden');
            }
        });
        
        // Handle suggestion clicks
        suggestionsDiv.addEventListener('click', (e) => {
            if (e.target.classList.contains('suggestion-item')) {
                const tag = e.target.dataset.tag;
                const words = tagsInput.value.split(' ');
                words[words.length - 1] = tag;
                tagsInput.value = words.join(' ') + ' ';
                tagsInput.focus();
                suggestionsDiv.classList.add('hidden');
            }
        });
        
        // Hide suggestions when clicking outside
        document.addEventListener('click', (e) => {
            if (!tagsInput.contains(e.target) && !suggestionsDiv.contains(e.target)) {
                suggestionsDiv.classList.add('hidden');
            }
        });
    } catch (err) {
        console.error('Failed to load recent tags:', err);
    }
}

async function handleUpload(e) {
    e.preventDefault();
    
    // Check if we're in multi-photo mode
    if (window.multiPhotoData) {
        return handleMultiPhotoUpload();
    }
    
    // Single photo upload
    const form = e.target;
    const metadata = {
        path: currentPhotoMetadata.path,
        title: form.title.value.trim(),
        alt: form.alt.value.trim(),
        description: form.description.value.trim(),
        tags: form.tags.value.split(/\s+/).filter(t => t),
        format: form.format.value,
        private: form.private.checked,
        mastodonEnabled: form['mastodon-enabled'].checked,
        mastodonText: form['mastodon-text'].value.trim(),
        mastodonVisibility: form['mastodon-visibility'].value,
        blueskyEnabled: form['bluesky-enabled'].checked,
        blueskyText: form['bluesky-text'].value.trim()
    };
    
    // Show progress with appropriate message
    if (metadata.mastodonEnabled || metadata.blueskyEnabled) {
        showProgress('Processing...');
    } else {
        showProgress('Uploading...');
    }
    document.getElementById('error-message').classList.add('hidden');
    document.getElementById('success-message').classList.add('hidden');
    form.classList.add('disabled');
    
    try {
        const result = await window.go.main.App.Upload(metadata);
        if (result.success) {
            // Copy snippet to clipboard
            await navigator.clipboard.writeText(result.snippet);
            
            // Show appropriate success message based on duplicate status
            if (result.duplicate && result.forceAvailable) {
                // For duplicates, only show re-upload option if NO social media was requested
                if (metadata.mastodonEnabled || metadata.blueskyEnabled) {
                    // Social media was posted, don't offer re-upload
                    document.getElementById('progress').classList.add('hidden');
                    
                    let message = 'Used existing image.';
                    if (result.socialPostStatus) {
                        switch (result.socialPostStatus) {
                            case 'mastodon_success':
                                message += ' Posted to Mastodon!';
                                break;
                            case 'bluesky_success':
                                message += ' Posted to Bluesky!';
                                break;
                            case 'both_success':
                                message += ' Posted to both services!';
                                break;
                            case 'mastodon_failed':
                                message += ' Mastodon post failed.';
                                break;
                            case 'bluesky_failed':
                                message += ' Bluesky post failed.';
                                break;
                            case 'both_failed':
                                message += ' Social posts failed.';
                                break;
                            case 'mastodon_success_bluesky_failed':
                                message += ' Posted to Mastodon, Bluesky failed.';
                                break;
                        }
                    } else {
                        // No status info, but we know posting was attempted
                        if (metadata.mastodonEnabled && metadata.blueskyEnabled) {
                            message += ' Posted to social media.';
                        } else if (metadata.mastodonEnabled) {
                            message += ' Posted to Mastodon.';
                        } else {
                            message += ' Posted to Bluesky.';
                        }
                    }
                    message += '\n\nURL copied to clipboard.';
                    
                    showSuccess(message, 'duplicate');
                    setTimeout(() => {
                        window.runtime.Quit();
                    }, 2500);
                } else {
                    // No social media, show the re-upload option
                    showReuploadOption(metadata);
                }
            } else if (result.duplicate) {
                // Duplicate but can't re-upload (shouldn't happen but handle it)
                document.getElementById('progress').classList.add('hidden');
                showSuccess('Already uploaded! URL copied to clipboard.', 'duplicate');
                setTimeout(() => {
                    window.runtime.Quit();
                }, 2000);
            } else {
                // New upload
                showSuccess('Uploaded! Snippet copied to clipboard.');
                
                // Close after a short delay for new uploads
                setTimeout(() => {
                    window.runtime.Quit();
                }, 1500);
            }
        } else {
            showError(result.error || 'Upload failed');
            document.getElementById('progress').classList.add('hidden');
            form.classList.remove('disabled');
        }
    } catch (err) {
        showError('Upload error: ' + err);
        document.getElementById('progress').classList.add('hidden');
        form.classList.remove('disabled');
    }
}

// Handle multi-photo upload
async function handleMultiPhotoUpload() {
    // Save current photo data
    saveCurrentPhotoData();
    
    // Get social media settings from form
    const form = document.getElementById('upload-form');
    
    // Debug: Check form state
    console.log('DEBUG: Form disabled state:', form.classList.contains('disabled'));
    console.log('DEBUG: Mastodon checkbox checked:', form['mastodon-enabled'].checked);
    console.log('DEBUG: Bluesky checkbox checked:', form['bluesky-enabled'].checked);
    console.log('DEBUG: Mastodon text element:', document.getElementById('mastodon-text'));
    console.log('DEBUG: Mastodon text value directly:', document.getElementById('mastodon-text').value);
    console.log('DEBUG: Bluesky text element:', document.getElementById('bluesky-text'));
    console.log('DEBUG: Bluesky text value directly:', document.getElementById('bluesky-text').value);
    
    const socialPost = {
        mastodonEnabled: form['mastodon-enabled'].checked,
        mastodonText: form['mastodon-text'].value.trim(),
        mastodonVisibility: form['mastodon-visibility'].value,
        blueskyEnabled: form['bluesky-enabled'].checked,
        blueskyText: form['bluesky-text'].value.trim()
    };
    
    console.log('DEBUG: Social post data:', socialPost);
    console.log('DEBUG: Mastodon text field value:', form['mastodon-text'].value);
    
    // Show progress
    showProgress(`Uploading ${window.multiPhotoData.length} photos...`);
    form.classList.add('disabled');
    
    try {
        // Build the JSON structure as specified in the design brief
        const uploadData = {
            post: socialPost.mastodonText || socialPost.blueskyText || '',
            images: window.multiPhotoData.map(photo => ({
                path: photo.path || '',
                alt: photo.alt,
                title: photo.title || '',
                description: photo.description || '',
                isFromPhotos: photo.isFromPhotos || false,
                photosIndex: photo.photosIndex || 0,
                photosId: photo.photosId || ''
            })),
            tags: [], // Collect common tags if needed
            mastodon: socialPost.mastodonEnabled,
            bluesky: socialPost.blueskyEnabled,
            visibility: socialPost.mastodonVisibility || 'public',
            format: form.format.value || 'url'
        };
        
        console.log('Uploading multiple photos:', uploadData);
        console.log('DEBUG: uploadData.bluesky =', uploadData.bluesky);
        console.log('DEBUG: uploadData.mastodon =', uploadData.mastodon);
        console.log('DEBUG: uploadData.post =', uploadData.post);
        
        // Call backend method
        const result = await window.go.main.App.UploadMultiplePhotos(uploadData);
        
        console.log('DEBUG: Upload result:', result);
        
        if (result.success) {
            document.getElementById('progress').classList.add('hidden');
            
            // Handle results based on format
            const outputs = result.outputs || [];
            let clipboardContent = '';
            
            // Count duplicates and new uploads
            let duplicateCount = 0;
            let newUploadCount = 0;
            outputs.forEach(output => {
                if (output.duplicate) {
                    duplicateCount++;
                } else {
                    newUploadCount++;
                }
            });
            
            switch (uploadData.format) {
                case 'markdown':
                    clipboardContent = outputs.map(o => o.markdown || o.url).join('\n');
                    break;
                case 'html':
                    clipboardContent = outputs.map(o => o.html || `<img src="${o.url}" alt="${o.alt}">`).join('\n');
                    break;
                case 'json':
                    clipboardContent = JSON.stringify(outputs, null, 2);
                    break;
                default: // 'url'
                    clipboardContent = outputs.map(o => o.url).join('\n');
                    break;
            }
            
            // Copy to clipboard
            if (clipboardContent) {
                await navigator.clipboard.writeText(clipboardContent);
            }
            
            // Build success message based on what was uploaded
            let successMessage = '';
            if (duplicateCount > 0 && newUploadCount > 0) {
                successMessage = `Uploaded ${newUploadCount} new photo${newUploadCount > 1 ? 's' : ''} and found ${duplicateCount} duplicate${duplicateCount > 1 ? 's' : ''}!`;
            } else if (duplicateCount > 0) {
                successMessage = `All ${duplicateCount} photo${duplicateCount > 1 ? 's were' : ' was'} already uploaded (duplicate${duplicateCount > 1 ? 's' : ''})!`;
            } else {
                successMessage = `Successfully uploaded ${newUploadCount} photo${newUploadCount > 1 ? 's' : ''}!`;
            }
            successMessage += ' URLs copied to clipboard.';
            
            if (result.socialStatus) {
                successMessage += ` ${result.socialStatus}.`;
            }
            
            // Show as duplicate type if all were duplicates
            const messageType = (duplicateCount > 0 && newUploadCount === 0) ? 'duplicate' : 'normal';
            showSuccess(successMessage, messageType);
            setTimeout(() => window.runtime.Quit(), 2000);
        } else {
            throw new Error(result.error || 'Upload failed');
        }
    } catch (err) {
        showError('Multi-photo upload error: ' + err);
        document.getElementById('progress').classList.add('hidden');
        form.classList.remove('disabled');
    }
}

function showError(message) {
    const errorDiv = document.getElementById('error-message');
    const contentDiv = errorDiv.querySelector('.overlay-content');
    
    contentDiv.innerHTML = `
        <div style="margin-bottom: 16px;">
            ${message}
        </div>
        <div style="margin-top: 20px;">
            <button type="button" onclick="document.getElementById('error-message').classList.add('hidden')">Dismiss</button>
            <button type="button" onclick="window.runtime.Quit()" style="margin-left: 8px;">Quit</button>
        </div>
    `;
    
    errorDiv.classList.remove('hidden');
}

function showProgress(message) {
    const progressDiv = document.getElementById('progress');
    const progressText = document.getElementById('progress-text');
    if (progressText) {
        progressText.textContent = message;
    }
    progressDiv.classList.remove('hidden');
}

function showSuccess(message, type = 'normal') {
    const successDiv = document.getElementById('success-message');
    const contentDiv = successDiv.querySelector('.overlay-content');
    
    // Remove duplicate class from overlay first
    successDiv.classList.remove('duplicate');
    
    if (type === 'duplicate') {
        successDiv.classList.add('duplicate');
        // Add a duplicate indicator icon and handle multi-line messages
        const formattedMessage = message.replace(/\n/g, '<br>');
        contentDiv.innerHTML = '<span class="duplicate-icon">↻</span> ' + formattedMessage;
    } else {
        // Handle multi-line messages for normal uploads too
        if (message.includes('\n')) {
            contentDiv.innerHTML = message.replace(/\n/g, '<br>');
        } else {
            contentDiv.textContent = message;
        }
    }
    
    successDiv.classList.remove('hidden');
}

function showInfo(message) {
    const successDiv = document.getElementById('success-message');
    const contentDiv = successDiv.querySelector('.overlay-content');
    contentDiv.textContent = message;
    successDiv.classList.remove('hidden');
    // Auto-hide after 3 seconds
    setTimeout(() => {
        successDiv.classList.add('hidden');
    }, 3000);
}

function showToast(message) {
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.textContent = message;
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.remove();
    }, 2000);
}

function showReuploadOption(metadata) {
    // Hide progress spinner
    document.getElementById('progress').classList.add('hidden');
    
    // Show the success overlay with re-upload option
    const successDiv = document.getElementById('success-message');
    const contentDiv = successDiv.querySelector('.overlay-content');
    
    successDiv.classList.add('duplicate');
    
    contentDiv.innerHTML = `
        <div style="margin-bottom: 16px;">
            <span class="duplicate-icon">↻</span> Already uploaded! URL copied to clipboard.
        </div>
        <div style="margin-top: 20px;">
            <button type="button" id="reupload-btn" class="reupload-button" style="margin-right: 8px;">Re-upload Anyway</button>
            <button type="button" id="done-btn" onclick="window.runtime.Quit()">Done</button>
        </div>
    `;
    
    successDiv.classList.remove('hidden');
    
    // Add click handler for re-upload
    document.getElementById('reupload-btn').onclick = async () => {
        successDiv.classList.add('hidden');
        await handleForceUpload(metadata);
    };
}

async function handleForceUpload(metadata) {
    const form = document.getElementById('upload-form');
    
    // Show progress
    showProgress('Re-uploading...');
    document.getElementById('error-message').classList.add('hidden');
    document.getElementById('success-message').classList.add('hidden');
    form.classList.add('disabled');
    
    try {
        const result = await window.go.main.App.ForceUpload(metadata);
        if (result.success) {
            // Copy snippet to clipboard
            await navigator.clipboard.writeText(result.snippet);
            
            // Hide progress
            document.getElementById('progress').classList.add('hidden');
            
            // Show success message
            showSuccess('Re-uploaded! Snippet copied to clipboard.');
            
            // Close after a short delay
            setTimeout(() => {
                window.runtime.Quit();
            }, 1500);
        } else {
            showError(result.error || 'Re-upload failed');
            document.getElementById('progress').classList.add('hidden');
            form.classList.remove('disabled');
        }
    } catch (err) {
        showError('Re-upload error: ' + err);
        document.getElementById('progress').classList.add('hidden');
        form.classList.remove('disabled');
    }
}


// Show UI for multiple photo selection
function showMultiPhotoUI(photos) {
    // Store photos for later use, preserving existing metadata
    window.multiPhotoData = photos.map((photo, index) => ({
        ...photo,
        index,
        alt: photo.alt || '',  // Preserve existing alt text
        description: photo.description || '',  // Preserve existing description
        hasAltText: !!photo.alt  // Set based on whether alt already exists
    }));
    window.currentPhotoIndex = 0;
    
    // Hide single photo preview, show multi-photo list
    document.getElementById('preview-container').classList.add('hidden');
    document.getElementById('multi-photo-list').classList.remove('hidden');
    
    // Update photo count
    document.getElementById('photo-count').textContent = photos.length;
    
    // Build photo list
    const listContainer = document.getElementById('photo-list-container');
    listContainer.innerHTML = photos.map((photo, index) => {
        const hasAlt = !!photo.alt;
        return `
        <div class="photo-item ${index === 0 ? 'selected' : ''}" data-index="${index}">
            <div class="thumbnail">
                <div class="thumbnail-placeholder">
                    <span class="loading-spinner"></span>
                </div>
            </div>
            <div class="photo-details">
                <div class="photo-name">${photo.photosFilename || photo.path.split('/').pop()}</div>
                <div class="photo-source">${photo.isFromPhotos ? 'Photos.app' : 'Finder'}</div>
            </div>
            <div class="alt-check ${hasAlt ? 'filled' : 'empty'}" title="${hasAlt ? 'Alt text provided' : 'Alt text not provided'}">${hasAlt ? '✓' : '○'}</div>
        </div>
    `}).join('');
    
    // Add click handlers for photo selection
    listContainer.querySelectorAll('.photo-item').forEach(item => {
        item.addEventListener('click', () => selectPhoto(parseInt(item.dataset.index)));
    });
    
    // Update upload button
    const uploadBtn = document.getElementById('upload-btn');
    uploadBtn.textContent = `Upload ${photos.length} Photos`;
    
    // Load first photo's details
    loadPhotoDetails(0);
    
    // Hide loading overlay
    document.getElementById('loading-overlay').classList.add('hidden');
    
    // Start loading thumbnails
    loadThumbnailsAsync(photos);
}

// Select a photo from the list
function selectPhoto(index) {
    // Save current photo data before switching
    saveCurrentPhotoData();
    
    // Update selection
    document.querySelectorAll('.photo-item').forEach(item => {
        item.classList.toggle('selected', parseInt(item.dataset.index) === index);
    });
    
    // Load new photo details
    window.currentPhotoIndex = index;
    loadPhotoDetails(index);
}

// Load details for a specific photo
function loadPhotoDetails(index) {
    const photo = window.multiPhotoData[index];
    
    // Update form fields
    document.getElementById('title').value = photo.title || '';
    document.getElementById('alt').value = photo.alt || '';
    document.getElementById('description').value = photo.description || '';
    document.getElementById('tags').value = (photo.tags || []).join(' ');
    
    // Focus on alt text if empty
    if (!photo.alt) {
        document.getElementById('alt').focus();
    }
}

// Save current photo data
function saveCurrentPhotoData() {
    if (window.multiPhotoData && window.currentPhotoIndex !== undefined) {
        const current = window.multiPhotoData[window.currentPhotoIndex];
        current.title = document.getElementById('title').value;
        current.alt = document.getElementById('alt').value;
        current.description = document.getElementById('description').value;
        current.tags = document.getElementById('tags').value.split(' ').filter(t => t);
        current.hasAltText = !!current.alt;
        
        // Update checkmark
        const photoItem = document.querySelector(`.photo-item[data-index="${window.currentPhotoIndex}"]`);
        if (photoItem) {
            const altCheck = photoItem.querySelector('.alt-check');
            if (altCheck) {
                if (current.hasAltText) {
                    altCheck.classList.remove('empty');
                    altCheck.classList.add('filled');
                    altCheck.textContent = '✓';
                    altCheck.title = 'Alt text provided';
                } else {
                    altCheck.classList.remove('filled');
                    altCheck.classList.add('empty');
                    altCheck.textContent = '○';
                    altCheck.title = 'Alt text not provided';
                }
            }
        }
    }
}

// Load thumbnails asynchronously
function loadThumbnailsAsync(photos) {
    // Clean up any existing listeners
    cleanupMultiPhotoListeners();
    
    // Listen for thumbnail-ready events
    thumbnailEventOff = runtime.EventsOn('thumbnail-ready', (data) => {
        console.log('Thumbnail ready for index:', data.index);
        
        const photoItem = document.querySelector(`.photo-item[data-index="${data.index}"]`);
        if (!photoItem) {
            console.error('Photo item not found for index:', data.index);
            return;
        }
        
        if (data.error) {
            console.error('Thumbnail error:', data.error);
            const thumbnailDiv = photoItem.querySelector('.thumbnail');
            if (thumbnailDiv) {
                thumbnailDiv.innerHTML = `<div class="thumbnail-error">Failed</div>`;
            }
            return;
        }
        
        if (data.thumbnail) {
            const thumbnailDiv = photoItem.querySelector('.thumbnail');
            if (thumbnailDiv) {
                thumbnailDiv.innerHTML = `<img src="${data.thumbnail}" alt="Thumbnail">`;
            }
        }
    });
    
    // Listen for metadata updates
    metadataEventOff = runtime.EventsOn('metadata-ready', (data) => {
        console.log('Metadata ready for index:', data.index);
        
        // Update stored photo data
        if (window.multiPhotoData && window.multiPhotoData[data.index]) {
            const photo = window.multiPhotoData[data.index];
            if (data.alt && !photo.alt) photo.alt = data.alt;
            if (data.title && !photo.title) photo.title = data.title;
            if (data.description && !photo.description) photo.description = data.description;  // Add description
            if (data.keywords && !photo.tags) photo.tags = data.keywords;
            
            // Update hasAltText flag
            photo.hasAltText = !!photo.alt;
            
            // Update the checkmark in the UI
            const photoItem = document.querySelector(`.photo-item[data-index="${data.index}"]`);
            if (photoItem && photo.hasAltText) {
                const altCheck = photoItem.querySelector('.alt-check');
                if (altCheck) {
                    altCheck.classList.remove('empty');
                    altCheck.classList.add('filled');
                    altCheck.textContent = '✓';
                    altCheck.title = 'Alt text provided';
                }
            }
            
            // If this is the currently selected photo, update the form
            if (data.index === window.currentPhotoIndex) {
                loadPhotoDetails(data.index);
            }
        }
    });
    
    // Now that listeners are set up, start the thumbnail generation
    console.log('Starting thumbnail generation for', photos.length, 'photos');
    window.go.main.App.StartThumbnailGeneration(photos).then(() => {
        console.log('Thumbnail generation started');
    }).catch(err => {
        console.error('Failed to start thumbnail generation:', err);
    });
}

// Clean up multi-photo event listeners
function cleanupMultiPhotoListeners() {
    if (thumbnailEventOff) {
        thumbnailEventOff();
        thumbnailEventOff = null;
    }
    if (metadataEventOff) {
        metadataEventOff();
        metadataEventOff = null;
    }
}
