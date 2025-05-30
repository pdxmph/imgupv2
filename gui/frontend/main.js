// Store current photo metadata globally
let currentPhotoMetadata = null;

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', async () => {
    try {
        // Load selected photo metadata
        const metadata = await window.go.main.App.GetSelectedPhoto();
        if (metadata && metadata.path) {
            currentPhotoMetadata = metadata;
            populateForm(metadata);
            loadPreview(metadata.path);
        } else {
            showError('No photo selected in Finder');
            setTimeout(() => {
                window.runtime.Quit();
            }, 2000);
            return;
        }
    } catch (err) {
        console.error('Failed to get selection:', err);
        showError('Failed to get selected photo: ' + err);
        return;
    }
    
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
});

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
        private: form.private.checked
    };
    
    // Show progress
    document.getElementById('progress').classList.remove('hidden');
    document.getElementById('error-message').classList.add('hidden');
    document.getElementById('success-message').classList.add('hidden');
    form.classList.add('disabled');
    
    try {
        const result = await window.go.main.App.Upload(metadata);
        if (result.success) {
            // Copy snippet to clipboard
            await navigator.clipboard.writeText(result.snippet);
            
            // Show success message
            showSuccess('Uploaded! Snippet copied to clipboard.');
            
            // Close after a short delay
            setTimeout(() => {
                window.runtime.Quit();
            }, 1500);
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
    errorDiv.textContent = message;
    errorDiv.classList.remove('hidden');
}

function showSuccess(message) {
    const successDiv = document.getElementById('success-message');
    successDiv.textContent = message;
    successDiv.classList.remove('hidden');
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
