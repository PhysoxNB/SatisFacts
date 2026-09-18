    // The Go backend embeds the report data as gzip-compressed JSON in a
    // hidden <div> as base64. We decode it here using the browser's native
    // DecompressionStream API (no external library needed).
    async function loadData() {
        var el = document.getElementById('embedded-data');
        var b64 = el.textContent.trim();
        var binary = atob(b64);
        var bytes = new Uint8Array(binary.length);
        for (var i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);

        // DecompressionStream is relatively new (Chrome 80+, FF 113+, Safari 16.4+).
        // Older browsers won't be able to load the report at all.
        if (typeof DecompressionStream === 'undefined') {
            throw new Error('Your browser does not support DecompressionStream. Please use Chrome 80+, Firefox 113+, or Safari 16.4+.');
        }

        var ds = new DecompressionStream('gzip');
        var blob = new Blob([bytes]);
        var stream = blob.stream().pipeThrough(ds);
        var text = await new Response(stream).text();
        return JSON.parse(text);
    }