<div align="center">
  <img src="assets/logo.png" alt="Local Content Share Logo" width="200">
  <h1>Local Content Share</h1>

  <a href="https://github.com/tanq16/local-content-share/actions/workflows/binary-build.yml"><img alt="Build Workflow" src="https://github.com/tanq16/local-content-share/actions/workflows/binary-build.yml/badge.svg"></a>&nbsp;<a href="https://github.com/tanq16/local-content-share/actions/workflows/docker-publish.yml"><img alt="Container Workflow" src="https://github.com/tanq16/local-content-share/actions/workflows/docker-publish.yml/badge.svg"></a><br>
  <a href="https://github.com/Tanq16/local-content-share/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/local-content-share"></a>&nbsp;<a href="https://hub.docker.com/r/tanq16/local-content-share"><img alt="Docker Pulls" src="https://img.shields.io/docker/pulls/tanq16/local-content-share"></a><br><br>
  <a href="#screenshots">Screenshots</a> &bull; <a href="#installation-and-usage">Install & Use</a> &bull; <a href="#tips-and-notes">Tips & Notes</a> &bull; <a href="#acknowledgements">Acknowledgements</a>
</div>

---

A simple & elegant self-hosted app for **storing/sharing text snippets and files** in your **local network** with **no setup on client devices**. Think of this as an *all-in-one alternative* to **airdrop**, **local-pastebin**, and a **scratchpad**. The primary features are:

- Make plain text **snippets** available to **view/share** on any device in the local network
- **Upload files** and make them available to **view/download** on any device in the local network
- Built-in **Notepad** with both **Markdown** and **Rich Text** editing capabilities
- **Rename** text snippets and files uploaded to easily find them in the UI
- **Edit** text snippets to modify their content as needed
- **Multi-file** **drag-n-drop** (drop into the text area) support for uploading files
- Configurable **expiration (or TTL, i.e., time to live)** per file/snippet for Never, 1 hour, 4 hours, 1 day, or Custom
- Use of **SSE** to automatically inform all clients of new/deleted/edited files
- Completely **local assets**, so the app works in your network even without internet
- **Multi-arch** (x86-64 and ARM64) **Docker image** for **homelab** deployments
- Frontend available over **browsers** and as a **PWA** (progressive web apps)
- Clean, modern interface with **automatic light/dark** UI that looks good on mobile too

Make sure to look into [Tips & Notes](#tips-and-notes) if you have questions about individual functionalities.

> [!NOTE]
> This application is meant to be deployed within your homelab only. There is no authentication mechanism implemented. If you are exposing to the public, ensure there is authentication fronting it and non-destructive users using it.

## Screenshots

| | Desktop View | Mobile View |
| --- | --- | --- |
| Light | <img src="assets/dlight.png" alt="Light"> | <img src="assets/mlight.png" alt="Light"> |
| Dark | <img src="assets/ddark.png" alt="Dark"> | <img src="assets/mdark.png" alt="Dark"> |

<details>
<summary>Expand for more screenshots</summary>

| Desktop View | Mobile View |
| --- | --- |
| <img src="assets/dmdlight.png"> | <img src="assets/mmdlight.png"> |
| <img src="assets/dmddark.png"> | <img src="assets/mmddark.png"> |
| <img src="assets/dmdrlight.png"> | <img src="assets/mmdrlight.png"> |
| <img src="assets/dmdrdark.png"> | <img src="assets/mmdrdark.png"> |
| <img src="assets/drtextlight.png"> | <img src="assets/mrtextlight.png"> |
| <img src="assets/drtextdark.png"> | <img src="assets/mrtextdark.png"> |
| <img src="assets/dsnippetlight.png"> | <img src="assets/msnippetlight.png"> |
| <img src="assets/dsnippetdark.png"> | <img src="assets/msnippetdark.png"> |

</details>

## Installation and Usage

### Using Docker (Recommended for Self-Hosting)

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

The application will be available at `http://localhost:8080` (or your server IP).

You can also use the following compose file with container managers like Portainer and Dockge (remember to change the mounted volume):

```yaml
services:
  contentshare:
    image: tanq16/local-content-share:main
    container_name: local-content-share
    volumes:
      - /home/tanq/lcshare:/app/data # Change as needed
    ports:
      - 8080:8080
```

### Using Binary

Download the appropriate binary for your system from the [latest release](https://github.com/tanq16/local-content-share/releases/latest).

Make the binary executable (for Linux/macOS) with `chmod +x local-content-share-*` and then run the binary with `./local-content-share-*`. The application will be available at `http://localhost:8080`.

### Local development

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
   - Type or paste your text in the text area (the upload button will change to a submit button)
   - (OPTIONAL) type the name of the snippet (otherwise it will name it as a time string)
   - Click the submit button (looks like the telegram arrow) to upload the snippet
- To rename files or text snippets:
   - Click the cursor (i-beam) icon and provide the new name
   - It will automatically prepend 4 random digits if the name isn't unique
- To edit existing snippets:
   - Click the pen icon and it will populate the expandable text area with the content
   - Write the new content and click accept or deny (check or cross) in the same text area
   - On accepting, it will edit the content; on denying, it will refresh the page
- To share files:
   - Click the upload button and select your file
   - OR drag and drop your file (even multiple files) to the text area
   - It will automatically append 4 random digits if filename isn't unique
- To view content, click the eye icon:
   - For text content, it shows the raw text, which can be copied with a button on top
   - For files, it shows raw text, images, PDFs, etc. (basically whatever the browser will do)
- To download files, click the download icon
- To delete content, click the trash icon
- To set expiration for a file or snippet
   - Click the clock icon with the "Never" text (signifying no expiry) to cycle between times
   - For a non-"Never" expiration, the file will automatically be removed after the specified period
   - Set the cycling button to 1 hour, 4 hours, 1 day, or Custom before adding a snippet or file
      - The Custom option will prompt to ask for the expiry after you click submit/upload
      - The value for custom expiration can be of the format `NT` (eg. `34m`, `3w`, `2M`, `11d`)
      - N is the number and T is the time denomination (m=minute, h=hour, d=day, w=week, M=month, y=year)
   - Use the `DEFAULT_EXPIRY` environment variable to set a default expiration
      - This value will be set as default on the home page instead of `Never`
      - The other options will still be available by cycling if needed
- The Notepad is for writing something quickly and getting back to it from any device
   - It supports both markdown and richtext modes
   - Content is automatically saved upon inactivity in the backend and will load as is on any device

### A Note on Reverse Proxies

Reverse proxies are fairly common in homelab settings to assign SSL certificates

### Backend Data Structure

The application creates a `data` directory to store all uploaded files, uploaded text snippets, notepad notes (in `files`, `text`, and `notepad` subfolders respectively). File expirations are saved in an `expiration.json` file in the data directory. Make sure the application has write permissions for the directory where it runs.

## Acknowledgements

The following people have contributed to the project:

- [TheArktect](https://github.com/TheArktect) - Added CLI argument for listen address.
- A lot of other users who created feature requests via GitHub issues.
