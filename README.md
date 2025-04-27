<div align="center">
  <img src="assets/logo.png" alt="Local Content Share Logo" width="200">
  <h1>Local Content Share</h1>

  <a href="https://github.com/tanq16/local-content-share/actions/workflows/binary-build.yml"><img alt="Build Workflow" src="https://github.com/tanq16/local-content-share/actions/workflows/binary-build.yml/badge.svg"></a>&nbsp;<a href="https://github.com/tanq16/local-content-share/actions/workflows/docker-publish.yml"><img alt="Container Workflow" src="https://github.com/tanq16/local-content-share/actions/workflows/docker-publish.yml/badge.svg"></a><br>
  <a href="https://github.com/Tanq16/local-content-share/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/local-content-share"></a>&nbsp;<a href="https://hub.docker.com/r/tanq16/local-content-share"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/tanq16/local-content-share"></a><br><br>
  <a href="#screenshots">Screenshots</a> &bull; <a href="#installation-and-usage">Install & Use</a> &bull; <a href="#tips-and-notes">Tips & Notes</a>
</div>

---

A simple self-hosted app for sharing text snippets and files within your local network across any device and a notepad. Think of this as a simple and elegant alternative to airdrop, local-pastebin and notepad. The primary features are:

- Make plain text snippets available to view/share on any device in the local network
- Upload files and make them available to view/download on any device in the local network
- Built-in Notepad with both Markdown and Rich Text editing capabilities
- Rename text snippets and files uploaded to easily find them in the UI
- Edit text snippets to modify their content as needed
- Multi-file drag-n-drop (drop into the text area) support for uploading files

From a technology perspective, the app boasts the following:

- Pure HTTP API, i.e., *no use of websockets* - this is good because it means *no external communications needed* for the sharing aspect
- Available as a binary for MacOS, Windows, and Linux for both x86-64 and ARM64 architectures
- Multi-arch (x86-64 and ARM64) Docker image for homelab deployments
- Works well with reverse proxies in the mix (tested with Cloudflare tunnels and Nginx Proxy Manager)
- Frontend available over browsers and as a PWA (so it shows as an app with an icon on the mobile home screens)
- Clean, modern interface with dark mode support that looks good on mobile too

> [!NOTE]
> This application is meant to be deployed within your homelab only. There is no authentication mechanism implemented. If you are exposing to the public, ensure there is authentication fronting it and non-destructive users using it.

## Screenshots

| | Desktop View | Mobile View |
| --- | --- | --- |
| Light | <img src="assets/desktop-light.png" alt="Desktop Light Mode"> | <img src="assets/mobile-light.png" alt="Mobile Light Mode"> |
| Dark | <img src="assets/desktop-dark.png" alt="Desktop Dark Mode"> | <img src="assets/mobile-dark.png" alt="Mobile Dark Mode"> |

Click [here](./assets/screencast.webm) to see a screencast of the app in action!

## Installation and Usage

### Using Docker (Recommended)

Use `docker` CLI one liner and setup a persistence directory (so a container failure does not delete your data):

```bash
mkdir $HOME/.localcontentshare
```
```bash
docker run --name local-content-share \
  -p 8080:8080 \
  -v $HOME/.localcontentshare:/app/data \
  tanq16/local-content-share:main
```

The application will be available at `http://localhost:8080`

You can also use the following compose file with container managers like Portainer and Dockge (remember to change the mounted volume):

```yaml
services:
  contentshare:
    image: tanq16/local-content-share:main
    container_name: local-content-share
    volumes:
      - /home/tanq/lcshare:/app/data
    ports:
      - 8080:8080
```

### Using Binary

Download the appropriate binary for your system from the [latest release](https://github.com/tanq16/local-content-share/releases/latest).

Make the binary executable (for Linux/macOS) with `chmod +x local-content-share-*` and then run the binary with `./local-content-share-*`. The application will be available at `http://localhost:8080`.

### Using Go

With `Go 1.23+` installed, run the following to download the binary to your GOBIN:

```bash
go install github.com/tanq16/local-content-share@latest
```

Or, you can build from source like so:

```bash
git clone https://github.com/tanq16/local-content-share.git && \
cd local-content-share && \
go build .
```

## Tips and Notes

- To share text content:
   - Type or paste your text in the text area
   - (OPTIONAL) type the name of the snippet (also the name of the file in the backend)
   - Click the send button (looks like the telegram arrow)
   - It will set a date-string-based file name if no custom name is specified
- To rename files or text snippets:
   - Click the cursor (i-beam) icon and provide the new name
   - It will automatically prepend 4 random digits if the name isn't unique
- To edit existing snippets:
   - Click the pen icon and it will populate the expandable text area with the content
   - Write the new content and click accept or deny (check or cross)
   - On accepting, it will edit the content; on denying, it will refresh the page
- To share files:
   - Click the upload button and select your file
   - OR drag and drop your file (even multiple files) to the text area
   - It will automatically append 4 random digits if filename isn't unique
- To view content, click the eye icon:
   - For text content, it shows raw text
   - For files, it shows raw text, images, PDFs, etc. (basically whatever way browsers can show items)
- To download files, click the download icon
- To delete content, click the trash icon
- The Notepad is only to write something quickly while leaving the tab open
   - The markdown mode can be used to render things as HTML or print as a PDF
   - There are some other features planned for improving the notepad functionality

A quick note of the data structure: the application creates a `data` directory to store all uploaded files and text content (both in `files` and `text` subfolders respectively). Make sure the application has write permissions for the directory where it runs.
