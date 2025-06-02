// Store current photo metadata globally
let currentPhotoMetadata = null;

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', async () => {
    // Initial load
    await loadSelectedPhoto();
    
    // Set up tag autocomplete
    setupTagAutocomplete();
    
    // Handle form submission
    document.getElementById('upload-form').onsubmit = handleUpload;
    
    // Handle cancel button
    document.getElementById('cancel-btn').onclick = () => {
        window.runtime.Quit();
    };
    
    // Handle Escape key
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            window.runtime.Quit();
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
});

// Add this function to watch for metadata population
function watchForMetadata() {
    const overlay = document.getElementById('loading-overlay');
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
    
    // Fallback: hide after 5 seconds regardless
    setTimeout(() => {
        clearInterval(checkInterval);
        overlay.classList.add('fade-out');
        setTimeout(() => {
            overlay.classList.add('hidden');
        }, 300);
    }, 5000);
}

// Extract photo loading logic into a separate function
async function loadSelectedPhoto() {
    try {
        // Start watching for metadata immediately
        watchForMetadata();
        
        // Clear any previous errors
        document.getElementById('error-message').classList.add('hidden');
        document.getElementById('success-message').classList.add('hidden');
        
        // Show the form
        document.getElementById('upload-form').classList.remove('hidden');
        
        // Load selected photo metadata
        const metadata = await window.go.main.App.GetSelectedPhoto();
        if (metadata && metadata.path) {
            currentPhotoMetadata = metadata;
            populateForm(metadata);
            loadPreview(metadata.path);
            
            // Show a note if this is from Photos
            if (metadata.isTemporary) {
                showInfo('Photo exported from Photos.app with metadata preserved');
            }
            
            // Focus on first editable field
            document.getElementById('title').focus();
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
    }
}

function populateForm(metadata) {
    document.getElementById('title').value = metadata.title || '';
    document.getElementById('alt').value = metadata.alt || '';
    document.getElementById('description').value = metadata.description || '';
    document.getElementById('tags').value = (metadata.tags || []).join(' ');
    document.getElementById('format').value = metadata.format || 'markdown';
    document.getElementById('private').checked = metadata.private || false;
}

function loadPreview(path) {
    const preview = document.getElementById('preview');
    const container = document.getElementById('preview-container');
    
    // Create a file URL for the image
    preview.src = 'file://' + path;
    container.classList.remove('hidden');
    
    preview.onerror = () => {
        container.classList.add('hidden');
    };
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
