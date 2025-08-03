#!/bin/bash

# Create necessary directories
mkdir -p static/fontawesome/webfonts
mkdir -p static/fontawesome/css
mkdir -p static/fonts
mkdir -p static/js
mkdir -p static/css

# Download Tailwind CSS
curl -sL "https://cdn.tailwindcss.com" -o "static/js/tailwindcss.js"

# Download Font Awesome 6.7.2
echo "Downloading Font Awesome..."
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/css/all.min.css -o static/fontawesome/css/all.min.css
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-brands-400.woff2 -o static/fontawesome/webfonts/fa-brands-400.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-regular-400.woff2 -o static/fontawesome/webfonts/fa-regular-400.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-solid-900.woff2 -o static/fontawesome/webfonts/fa-solid-900.woff2
curl -sL https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/fa-v4compatibility.woff2 -o static/fontawesome/webfonts/fa-v4compatibility.woff2

# Update CSS to use local webfonts
sed -i.bak 's|https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.7.2/webfonts/|/static/fontawesome/webfonts/|g' static/fontawesome/css/all.min.css
rm static/fontawesome/css/all.min.css.bak

# Download Inter font from Google Fonts
echo "Downloading Inter font..."
curl -sL "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap" -o static/css/inter.css

# Download font files referenced in the CSS
grep -o "https://fonts.gstatic.com/s/inter/[^)]*" static/css/inter.css | while read -r url; do
  filename=$(basename "$url")
  curl -sL "$url" -o "static/fonts/$filename"
done

# Update font CSS to use local files
sed -i.bak 's|https://fonts.gstatic.com/s/inter/v../|/static/fonts/|g' static/css/inter.css
rm static/css/inter.css.bak

# Download Highlight.js
echo "Downloading Highlight.js..."
curl -sL https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.7.0/highlight.min.js -o static/js/highlight.min.js

# Download Catppuccin theme for Highlight.js
echo "Downloading Catppuccin theme for Highlight.js..."
curl -sL https://cdn.jsdelivr.net/npm/@catppuccin/highlightjs@0.1.1/css/catppuccin-latte.css -o static/css/catppuccin-latte.css
curl -sL https://cdn.jsdelivr.net/npm/@catppuccin/highlightjs@0.1.1/css/catppuccin-mocha.css -o static/css/catppuccin-mocha.css

# Download Marked.js for markdown parsing
echo "Downloading Marked.js..."
curl -sL https://cdnjs.cloudflare.com/ajax/libs/marked/4.3.0/marked.min.js -o static/js/marked.min.js

echo "Assets downloaded and configured successfully!"
