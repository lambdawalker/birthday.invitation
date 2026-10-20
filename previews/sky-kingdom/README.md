# Sky kingdom parallax test

Standalone visual experiment; no build tools, API, credentials, or deployment required.
From the repository root:

```sh
python -m http.server 8000 --directory previews/sky-kingdom
```

Open http://localhost:8000. On Windows, `py -m http.server 8000 --directory previews/sky-kingdom` also works.
Scroll to move cloud/tree layers. The top Motion button disables parallax; OS reduced-motion settings are respected. RSVP controls only demonstrate states in memory: nothing is saved or sent. This folder is outside `web/` and is not deployed by Pulumi.

The castle/staircase and Abi figures share a stationary coordinate system. Clouds and tree groups move independently with bounded offsets. Text remains selectable HTML. The current art is a draft: transparent assets include soft painted halos, and scenery is vertically stretched to explore a long mobile journey. The two small Abi figures use different Viking outfits from the supplied illustrations.

`assets/` contains optimized WebP derivatives of the generated sky, castle/path, two character poses, cloud sheet, and tree sheet. They deliberately stay in ordinary Git so the preview also works from a source ZIP. Original high-resolution artwork should use Git LFS. No external fonts or runtime dependencies are loaded.
