#!/bin/bash

# Create directories
mkdir -p static/fontawesome/css
mkdir -p static/fontawesome/webfonts

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

echo "Assets downloaded and configured successfully!"
