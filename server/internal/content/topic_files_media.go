package content

import (
	"errors"
	"fmt"
)

// This file is topic 26 files-media (group media-realtime, curriculum
// order 26): the streaming doctrine for binary cargo — uploads are
// streams that never materialize, bounded by io.LimitReader and staged
// through temp-file-plus-rename (demo files:workbench walks the whole
// workbench live); multipart parsing with r.MultipartReader instead of
// the buffering ParseMultipartForm; downloads with honest
// Content-Disposition; RFC 7233 range requests as pure math (206
// slices, the 416 census, If-Range validators); backpressure as the
// property a streaming pipeline already IS; image processing with the
// stdlib codecs behind a decode-bomb pixel budget; and the three-lock
// path jail that keeps hostile names from becoming paths. The terminal
// companion closes with the curl tour: -F uploads and ranged curls.

// filesMediaTopic builds topic 26.
func filesMediaTopic() (Topic, error) {
	var buildErr error
	collect := func(what string, err error) {
		if err != nil {
			buildErr = errors.Join(buildErr, fmt.Errorf("%s: %w", what, err))
		}
	}
	text := func(id, body string) TextBlock {
		block, err := NewTextBlock(id, body)
		collect("text "+id, err)
		return block
	}
	code := func(id, language, caption, source, snippet string) CodeBlock {
		block, err := NewCodeBlock(id, language, caption, source, snippet)
		collect("code "+id, err)
		return block
	}
	demo := func(id, title, description string) DemoBlock {
		block, err := NewDemoBlock(id, title, description)
		collect("demo "+id, err)
		return block
	}
	list := func(id string, ordered bool, items ...string) ListBlock {
		block, err := NewListBlock(id, ordered, items)
		collect("list "+id, err)
		return block
	}
	section := func(id, title, lead string, blocks ...Block) Section {
		assembled, err := NewSection(id, title, lead, blocks)
		collect("section "+id, err)
		return assembled
	}

	sections := []Section{
		// ── 1. Uploads are streams ──────────────────────────────────
		section("uploads-are-streams", "Uploads are streams, not slurps",
			"An upload is an unbounded byte stream from a stranger. The one thing you must never do is materialize it — every correct file handler is a bounded copy from a reader to a writer.",
			text("streams-pitch",
				"Every topic before this one moved JSON — bodies small enough that reading them felt free. A file upload breaks that assumption in one move: the request body can be BYTES BEYOND MEMORY, arriving at network speed, from a client you do not control. The naive handler reads the whole body into a slice and then does something with it; the honest handler treats the body as what the type already says it is — an ``io.Reader`` — and copies it through a fixed-size buffer to its destination. Go's ``io.Copy`` is exactly that copy: a 32 KiB buffer, refilled from the reader and drained to the writer until EOF, so a 4 GiB upload costs 32 KiB of resident memory whether the file is 4 KiB or 4 GiB. The difference between a stream and a slurp is not performance polish — it is the difference between surviving your first large upload and being killed by it."),
			text("streams-bound",
				"Streaming alone is not safety: an unbounded copy still lets a hostile client push ``/dev/zero`` at you until the disk fills. Every hop in this topic's pipeline carries an explicit BOUND. The domain's ``Store`` port refuses to exist without one — ``SaveStream(ctx, name, body, limit)`` — because a limit is not configuration, it is part of what it means to accept an untrusted stream. The store enforces it with ``io.LimitReader`` so the refusal happens MID-STREAM, after the bytes that prove the violation have been read and before the next block is pulled. And at the HTTP edge the transport itself is wrapped in ``http.MaxBytesReader``, so a request larger than the object limit plus framing headroom dies at the door — the connection closes before the disk is asked for anything. Defense in depth on a byte budget: the edge bounds the request, the store bounds the object, and neither trusts the other."),
			code("streams-port-go", "go",
				"The store port: uploads arrive as readers under a stated limit, downloads leave as seekers so ranges can seek instead of re-reading.",
				"server/internal/domain/files/files.go",
				"type Store interface {\n"+
					"	// SaveStream reads body (already an untrusted stream) under name,\n"+
					"	// refusing with ErrTooLarge once more than limit bytes have been\n"+
					"	// read. The adapter streams to a sibling temp file and renames, so\n"+
					"	// a refused or crashed upload never leaves a partial object behind.\n"+
					"	SaveStream(ctx context.Context, name string, body io.Reader, limit int64) (Object, error)\n"+
					"	// Open returns the object's metadata plus a seeker over its bytes.\n"+
					"	// ErrNotFound when absent; ErrInvalidName when the name refuses or\n"+
					"	// escapes the root (including via symlink).\n"+
					"	Open(ctx context.Context, name string) (Object, io.ReadSeekCloser, error)\n"+
					"	// Stat returns the metadata without opening the bytes.\n"+
					"	Stat(ctx context.Context, name string) (Object, error)\n"+
					"}"),
			text("streams-landing",
				"How an upload LANDS matters as much as how it streams. Writing directly to the final path means a crash, a refused limit, or a cancelled context halfway through leaves a HALF-FILE under a real name — the worst kind of corruption, because it looks exactly like a small legitimate object. The local adapter stages every upload in a sibling temp file (``.incoming-*``) inside the store root and moves it to its final name with ``os.Rename``, which is atomic on POSIX: concurrent uploads of the same name never interleave, and a failure anywhere before the rename leaves nothing but a temp file the cleanup path removes. The ``limit+1`` trick is the refusal's proof — reading one byte PAST the limit is how the copy knows the stream exceeded it without ever trusting a ``Content-Length`` header the client could lie about:"),
			code("streams-save-go", "go",
				"The local adapter's staged save: bounded copy, mid-stream refusal, atomic landing.",
				"server/internal/data/files/local.go",
				"	tmp, err := os.CreateTemp(s.root, \".incoming-*\")\n"+
					"	if err != nil {\n"+
					"		return domfiles.Object{}, fmt.Errorf(\"files: stage upload: %w\", err)\n"+
					"	}\n"+
					"	// …tmpName captured, cleanup closes and removes on every failure path…\n"+
					"	// limit+1 lets one extra byte through: reading it PROVES the stream\n"+
					"	// exceeded the limit without ever trusting a Content-Length header.\n"+
					"	written, err := io.Copy(tmp, io.LimitReader(&ctxReader{ctx: ctx, r: body}, limit+1))\n"+
					"	if err != nil {\n"+
					"		return cleanup(fmt.Errorf(\"files: stream upload: %w\", err))\n"+
					"	}\n"+
					"	if written > limit {\n"+
					"		return cleanup(fmt.Errorf(\"%w: %d bytes against a %d-byte limit\",\n"+
					"			domfiles.ErrTooLarge, written, limit))\n"+
					"	}\n"+
					"	// …close the staged file, then land it atomically…\n"+
					"	if err := os.Rename(tmpName, target); err != nil {\n"+
					"		os.Remove(tmpName)\n"+
					"		return domfiles.Object{}, fmt.Errorf(\"files: land upload: %w\", err)\n"+
					"	}\n"+
					"	return s.Stat(ctx, clean)"),
			text("streams-cancel",
				"Cancellation is the third leg of the doctrine. A streaming copy that ignores its ``context.Context`` will happily drain a client that has given up — you pay disk and CPU for bytes nobody is waiting for anymore. The adapter's copy runs through a small ``ctxReader`` wrapper that checks the context before every ``Read``, so a cancelled or timed-out request stops the copy at the next buffer refill (and the staging discipline from the last block erases what it had written — cancelled uploads leave nothing). The workbench demo measures all of this live: it pushes a 4 MiB multipart body through a real ``io.Pipe`` and reports the process's total allocations for the exchange, which come out orders of magnitude below the payload — the number that proves the object never existed in memory."),
		),
		// ── 2. Multipart streaming ──────────────────────────────────
		section("multipart-streaming", "Multipart streaming: MultipartReader, never ParseMultipartForm",
			"The multipart body is a protocol on top of a stream. Parse it the buffering way and you pay the whole upload twice; parse it the streaming way and each part arrives as a reader.",
			text("multipart-pitch",
				"A file upload rides inside a ``multipart/form-data`` body — a framed format where boundaries separate fields, each with its own headers, one of which carries the file. Go's ``http.Request`` offers two ways to read it. ``ParseMultipartForm(maxMemory)`` is the convenient one: it parses EVERYTHING up front, buffering fields to memory and spilling file parts to temp files, and hands you ``*multipart.FileHeader`` values you open afterwards. It works — and it quietly rebuilds the slurp the last section dismantled: the whole body is consumed before your code sees a byte, memory and temp-disk costs are decided by the framework rather than by your limits, and a hostile client controls exactly how much. ``r.MultipartReader()`` is the streaming one: it returns a ``*multipart.Reader`` over the RAW body, and ``NextPart`` hands you each part as it arrives — an ``io.Reader`` you copy yourself, under your own bound, with your own refusal path."),
			code("multipart-save-go", "go",
				"The domain's multipart orchestration: parts stream from the raw body into the store — no buffering call anywhere.",
				"server/internal/domain/files/multipart.go",
				"	reader := multipart.NewReader(body, params[\"boundary\"])\n"+
					"	for {\n"+
					"		part, err := reader.NextPart()\n"+
					"		if errors.Is(err, io.EOF) {\n"+
					"			return Object{}, ErrNoFilePart\n"+
					"		}\n"+
					"		if err != nil {\n"+
					"			return Object{}, fmt.Errorf(\"files: read multipart part: %w\", err)\n"+
					"		}\n"+
					"		if part.FileName() == \"\" {\n"+
					"			// A non-file field: drain it and keep looking for the file.\n"+
					"			// The drain is bounded by the same limit as the upload — the\n"+
					"			// part's framing is structure, not a promise, and a text\n"+
					"			// field larger than the object limit is hostile input.\n"+
					"			skipped, err := io.Copy(io.Discard, io.LimitReader(part, limit+1))\n"+
					"			// …close the part, refuse when the field overran the limit…\n"+
					"			continue\n"+
					"		}\n"+
					"		object, err := store.SaveStream(ctx, part.FileName(), part, limit)\n"+
					"		part.Close()\n"+
					"		return object, err\n"+
					"	}"),
			text("multipart-edge",
				"The HTTP edge around this orchestration stays thin: wrap the body in ``http.MaxBytesReader`` (the object limit plus 1 MiB of multipart framing headroom — boundaries and part headers are bytes too), hand the content type and the raw body to the domain, and map the typed errors to the envelopes this guide has served since topic 04. One Go behavior worth knowing before you trust your own tests: ``Part.FileName()`` returns the path BASE — Go's multipart stack strips ``../`` and directory structure from the filename before you ever see it, so ``../escape.txt`` arrives as ``escape.txt``. That is a real defense, and it is also why this guide's sanitizer is defense in DEPTH rather than a first line: the store still refuses any name carrying structure, because the port's contract must hold for every caller, not just the ones who arrived through Go's parser:"),
			code("multipart-handler-go", "go",
				"The upload route: transport bounded at the edge, streamed by the domain, typed refusals mapped to 422s.",
				"server/internal/presentation/demo_files.go",
				"func (h *FilesUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n"+
					"	r.Body = http.MaxBytesReader(w, r.Body, h.limit+(1<<20))\n"+
					"	object, err := domfiles.SaveMultipart(r.Context(), h.store, r.Header.Get(\"Content-Type\"), r.Body, h.limit)\n"+
					"	if err != nil {\n"+
					"		// …ErrTooLarge, MaxBytesError, refused names, missing file part → 422…\n"+
					"		return\n"+
					"	}\n"+
					"	writeJSON(w, http.StatusCreated, filesUploadResponse{\n"+
					"		Name:        object.Name,\n"+
					"		Bytes:       object.Size,\n"+
					"		ContentType: object.ContentType,\n"+
					"	})\n"+
					"}"),
			demo("files:workbench", "The files workbench: one request, the whole streaming doctrine",
				"One request runs the topic's five beats against a REAL store. The streamed upload: a 4 MiB multipart body flows through a pipe into the store part by part, and the story reports the process's own allocations for the exchange — kilobytes against megabytes, the slurp test made measurable. The size bound: a stream that runs past its limit is refused mid-copy with nothing left behind. The ranged download: a mid-file slice served as 206 and checked byte-for-byte, a suffix range, a past-the-end range answered with the honest 416 census, and a stale If-Range falling back to the whole object. The traversal sweep: six hostile names refused by the jail, a symlink planted inside the root pointing outside it caught by EvalSymlinks, and the clean object still serving afterwards. The thumbnail: a bounded decode, a box-sample downscale with both encodings, and a 53-byte header claiming 30000×30000 pixels refused before a single pixel is allocated."),
		),
		// ── 3. Downloads and Content-Disposition ────────────────────
		section("downloads-and-disposition", "Downloads and Content-Disposition",
			"Serving a file back is the mirror of receiving one: stream from a seeker, name the bytes honestly, and never let a filename rewrite your response headers.",
			text("download-pitch",
				"A download handler is two concerns braided together. The BYTES are the easy half — the store's ``Open`` returns an ``io.ReadSeekCloser``, so the route is one ``io.Copy`` from the seeker to the ``ResponseWriter``, memory-flat for any object size. The NAME is where the bugs live, because the browser does not just save the bytes: it parses the ``Content-Disposition`` header to decide what to call the file, and that header is a protocol — quotes, backslashes, non-ASCII, and newlines all have meanings of their own. Serving the raw client-supplied filename inside the quoted clause is the classic header-injection bug: a filename containing a newline or a quote does not just break the save dialog, it changes the shape of your response. The honest header carries BOTH clauses the standard defines — an ASCII-only quoted fallback for ancient clients, and the RFC 5987/6266 ``filename*=UTF-8''…`` ext-value with the real name percent-encoded UTF-8:"),
			code("disposition-go", "go",
				"The domain's Content-Disposition encoder: escaped ASCII fallback plus the percent-encoded ext-value.",
				"server/internal/domain/files/files.go",
				"func ContentDisposition(filename string) string {\n"+
					"	var fallback strings.Builder\n"+
					"	for _, r := range filename {\n"+
					"		switch {\n"+
					"		case r == '\"' || r == '\\\\':\n"+
					"			fallback.WriteByte('\\\\')\n"+
					"			fallback.WriteRune(r)\n"+
					"		case r > 0x20 && r < 0x7F:\n"+
					"			fallback.WriteRune(r)\n"+
					"		default:\n"+
					"			fallback.WriteByte('_')\n"+
					"		}\n"+
					"	}\n"+
					"	return fmt.Sprintf(\"attachment; filename=\\\"%s\\\"; filename*=UTF-8''%s\",\n"+
					"		fallback.String(), url.PathEscape(filename))\n"+
					"}"),
			text("download-sniff",
				"The ``Content-Type`` on a download deserves the same distrust as the filename. The client's upload carried a content type claim; the store ignores it and SNIFFS the stored bytes instead — Go's ``http.DetectContentType`` reads the first 512 bytes and reports what the bytes actually look like (PNG magic, JPEG magic, text, or the honest ``application/octet-stream`` fallback). Serving metadata derived from the bytes closes the loop: what the client CLAIMED never reaches what the next client downloads. The sniff happens once at ``Open`` time and the seeker is rewound afterwards, so the route's range math always starts at offset zero no matter how far the sniffer read:"),
			code("sniff-go", "go",
				"Open sniffs from the stored bytes, then rewinds — metadata derived from evidence, position preserved for the caller.",
				"server/internal/data/files/local.go",
				"	// Sniff from the bytes, then rewind: the caller's Range math starts\n"+
					"	// at offset zero no matter how far DetectContentType read.\n"+
					"	head := make([]byte, 512)\n"+
					"	n, _ := io.ReadFull(file, head)\n"+
					"	if _, err := file.Seek(0, io.SeekStart); err != nil {\n"+
					"		file.Close()\n"+
					"		return domfiles.Object{}, nil, fmt.Errorf(\"files: rewind object: %w\", err)\n"+
					"	}\n"+
					"	return domfiles.Object{\n"+
					"		Name:        filepath.Base(path),\n"+
					"		Size:        info.Size(),\n"+
					"		ContentType: http.DetectContentType(head[:n]),\n"+
					"		ModTime:     info.ModTime().UTC(),\n"+
					"	}, file, nil"),
		),
		// ── 4. Range requests ───────────────────────────────────────
		section("range-requests", "Range requests: RFC 7233 in one pure function",
			"A Range header asks for a slice of the object. The whole protocol — 206 slices, clamped over-asks, the 416 census, If-Range validators — is arithmetic over two numbers you already have.",
			text("range-pitch",
				"Range requests are how video players seek, how download managers resume, and how cloud storage clients fetch shards — and the remarkable thing about them is that the server side is PURE MATH. The header ``Range: bytes=100-199`` names an inclusive byte slice; the server already knows the object's size; everything else follows. The protocol's three answers map exactly to three code paths: a satisfiable range serves ``206 Partial Content`` with a ``Content-Range`` naming the slice and the total, an unsatisfiable range (first byte past the end, or a zero-byte suffix) serves ``416`` with the census ``Content-Range: bytes */size`` so the client can re-ask sensibly, and a malformed header is IGNORED — the RFC is explicit that broken syntax gets the full 200, never an error, because a range is an optimization a client may get wrong without punishment. The header vocabulary is small enough to hold at once:"),
			list("range-vocabulary", false,
				"Range — the request's ask, one ``bytes=first-last`` inclusive slice, a ``bytes=start-`` open end, or a ``bytes=-N`` suffix for the last N bytes; anything else is ignored, not refused.",
				"Accept-Ranges: bytes — the server's advertisement that ranges are supported, stamped on every full 200 so clients learn they may ask.",
				"Content-Range — the 206's answer, ``bytes first-last/total``, naming EXACTLY which bytes ride in this body; the 416's census form ``bytes */total`` states the size instead.",
				"If-Range — the conditional gate: a strong ETag or an HTTP-date the object must still match, or the range is ignored and the whole object is served (a changed object must never hand out a stale slice).",
				"206 and 416 — the two range-specific statuses: Partial Content for a served slice, Not Satisfiable for an honest refusal that still tells the client the real size.",
			),
			code("range-parse-go", "go",
				"The range math: one pure function, three verdicts — slice, unsatisfiable, ignore.",
				"server/internal/domain/files/range.go",
				"func ParseRange(header string, size int64) (spec RangeSpec, unsatisfiable bool, present bool) {\n"+
					"	const unit = \"bytes=\"\n"+
					"	if !strings.HasPrefix(header, unit) {\n"+
					"		return RangeSpec{}, false, false\n"+
					"	}\n"+
					"	// …suffix ranges (\"bytes=-N\"), open ends, invalid syntax — ignored or clamped…\n"+
					"	if size == 0 || start >= size {\n"+
					"		return RangeSpec{}, true, true\n"+
					"	}\n"+
					"	if end >= size {\n"+
					"		end = size - 1 // a client may legitimately over-ask; clamp\n"+
					"	}\n"+
					"	return RangeSpec{Start: start, End: end}, false, true\n"+
					"}"),
			text("route-slice",
				"The route turns each verdict into its response, and the ORDER of operations is the teaching: seek BEFORE writing the status, because after ``WriteHeader`` a failure could only be logged, never re-shaped. A present, satisfiable range seeks the store's seeker to ``spec.Start``, stamps ``Content-Range`` and an exact ``Content-Length`` (the slice's length — a 206 whose Content-Length disagrees with its Content-Range is a lying response), writes ``206``, and copies exactly ``spec.Length()`` bytes through a ``io.LimitReader`` so a seeker positioned anywhere still serves precisely the promised slice. This store deliberately answers MULTI-RANGE requests (``bytes=0-99,200-299``) with the whole object rather than half an answer — a multipart/byteranges body is a real option this guide leaves as an exercise, and silently serving only the FIRST range is the bug the honest fallback exists to avoid:"),
			code("range-route-go", "go",
				"The download route's range paths: 416 census, exact 206 slice, malformed ignored.",
				"server/internal/presentation/demo_files.go",
				"	if rangeHeader := r.Header.Get(\"Range\"); rangeHeader != \"\" &&\n"+
					"		domfiles.HonorsIfRange(r.Header.Get(\"If-Range\"), etag, object.ModTime) {\n"+
					"		spec, unsatisfiable, present := domfiles.ParseRange(rangeHeader, object.Size)\n"+
					"		if present && unsatisfiable {\n"+
					"			w.Header().Set(\"Content-Range\", domfiles.UnsatisfiableContentRange(object.Size))\n"+
					"			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)\n"+
					"			return\n"+
					"		}\n"+
					"		if present {\n"+
					"			// Seek BEFORE the status is written: after WriteHeader a\n"+
					"			// failure could only be logged, not re-shaped.\n"+
					"			if _, err := content.Seek(spec.Start, io.SeekStart); err != nil {\n"+
					"				// …map to the internal envelope…\n"+
					"			}\n"+
					"			w.Header().Set(\"Content-Range\", domfiles.ContentRange(spec, object.Size))\n"+
					"			w.Header().Set(\"Content-Length\", strconv.FormatInt(spec.Length(), 10))\n"+
					"			w.WriteHeader(http.StatusPartialContent)\n"+
					"			if _, err := io.Copy(w, io.LimitReader(content, spec.Length())); err != nil {\n"+
					"				h.logger.Warn(\"files: range copy failed\", slog.Any(\"error\", err))\n"+
					"			}\n"+
					"			return\n"+
					"		}\n"+
					"		// Malformed or multi-range: fall through to the whole object.\n"+
					"	}"),
			text("range-validators",
				"If-Range deserves its own paragraph because it is the range protocol's only CONDITIONAL gate, and its rule is strict on purpose: a weak validator (``W/\"…\"``) never authorizes a range, a stale strong ETag never authorizes a range, and an HTTP-date only counts when it still matches the object's modification time to the second. The reasoning is correctness, not paranoia — a client holding bytes 0-99 of version A that asks for bytes 100-199 of what it hopes is still version A must receive either the new whole object or nothing partial, never a version-B slice welded onto version-A bytes. One Go-line detail this guide's own tests caught: ``http.ParseTime`` accepts exactly the three HTTP-date formats (IMF-fixdate in GMT, RFC850, asctime) — an RFC 1123 string with a ``UTC`` zone spelling is not among them, which is simply the standard being enforced: HTTP dates are GMT, and servers that send ``Last-Modified`` format them that way for exactly this comparison."),
		),
		// ── 5. Backpressure ─────────────────────────────────────────
		section("backpressure", "Backpressure: the pipeline you must not break",
			"Backpressure is not something you add to a streaming pipeline — it is what a streaming pipeline IS. Every bounded buffer in the chain pushes slowness upstream; every unbounded one quietly removes it.",
			text("backpressure-pitch",
				"The word sounds like a technique; it is actually a PROPERTY. When the disk is slower than the network, something must give: either the reader stops reading (and the TCP window closes, and the client's writes block — pressure travels all the way back to the sender), or some buffer in the middle swells to absorb the difference. A pipeline built from bounded pieces has backpressure for free: ``io.Copy`` pulls only when its 32 KiB buffer has room, an ``io.Pipe`` holds only its small kernel buffer, a store staging to disk applies the disk's own pace to every read it makes. The whole topic so far has been one backpressure-preserving chain — reader to buffer to disk — and the workbench demo's upload beat is deliberately built on an ``io.Pipe`` so the story can SHOW the writer blocking when the store is the slow one."),
			list("backpressure-chain", false,
				"io.Pipe — a synchronous in-memory conduit: every Write blocks until a Read consumes it, the kernel of honest flow control and the demo's generator-to-store link.",
				"io.Copy — pulls only when its fixed buffer drains, so a slow writer slows the reader with no extra machinery.",
				"io.LimitReader — the bound that turns an infinite stream into a finite one; refusal is flow control with teeth.",
				"http.MaxBytesReader — the edge's bound, closing the connection when the request overruns before any handler logic runs.",
				"ctxReader — the context guard inside the copy, so a cancelled reader stops the whole chain instead of draining a client that left.",
			),
			text("backpressure-break",
				"Backpressure is not broken loudly; it is broken quietly, one convenience at a time. ``io.ReadAll`` on an untrusted body — the slurp again, now as a flow-control hole: the reader never needs to pause because the slice absorbs everything. A goroutine-per-upload design with no concurrency cap — each goroutine's buffers are bounded, but the COUNT is not, so a thousand concurrent uploads are a thousand bounded buffers the machine cannot afford. An unbounded channel between a fast producer and a slow consumer — the channel grows without limit and the producer never learns the consumer is drowning. Every one of these has a bounded sibling: stream instead of slurp, a semaphore or worker pool instead of spawn-per-request (the goroutines topic's pattern), a buffered channel of size N with blocking sends. The discipline is to ask, for every queue in the system, the one question that matters: WHAT happens when it is full?"),
			code("backpressure-ctxreader-go", "go",
				"The copy's context guard: cancellation stops the chain at the next buffer refill.",
				"server/internal/data/files/local.go",
				"// ctxReader makes io.Copy cancellation-aware: each Read first checks\n"+
					"// the context, so a cancelled upload stops the copy (and the producer\n"+
					"// behind it, through backpressure) instead of draining to completion.\n"+
					"type ctxReader struct {\n"+
					"	ctx context.Context\n"+
					"	r   io.Reader\n"+
					"}\n"+
					"\n"+
					"// Read implements io.Reader with a context guard.\n"+
					"func (c *ctxReader) Read(p []byte) (int, error) {\n"+
					"	if err := c.ctx.Err(); err != nil {\n"+
					"		return 0, fmt.Errorf(\"files: upload cancelled: %w\", err)\n"+
					"	}\n"+
					"	n, err := c.r.Read(p)\n"+
					"	if err != nil && err != io.EOF && c.ctx.Err() != nil {\n"+
					"		return n, fmt.Errorf(\"files: upload cancelled: %w\", c.ctx.Err())\n"+
					"	}\n"+
					"	return n, err\n"+
					"}"),
			text("backpressure-measured",
				"The demo's generator is the other half of the proof. It writes the 4 MiB pattern through ``multipart.NewWriter(pw)`` in 64 KiB chunks — and because the pipe is synchronous, the generator's pace IS the store's pace: it cannot run ahead by more than one pipe buffer. When the store's part reader stops at the part's end (the store takes the first file part and returns), the generator's trailing writes fail with ``ErrClosedPipe`` — expected, handled, and reported as success, because the bytes that mattered all landed. That is what a correctly-shaped streaming demo looks like: both sides honest about who is waiting on whom, and the memory meter — kilobytes allocated for megabytes transferred — the receipt."),
		),
		// ── 6. Image processing ─────────────────────────────────────
		section("image-processing", "Image processing: stdlib codecs and the pixel budget",
			"Thumbnails are the file topic's transform: decode without trusting, downscale by hand, re-encode in the source format — all with the standard library and a budget that stops decode bombs before a pixel exists.",
			text("image-pitch",
				"Go's standard library ships a complete, interface-shaped imaging substrate: ``image.Decode`` turns registered-format bytes into an ``image.Image`` (a color model over a rectangle — ``At(x, y)`` returns a color), and ``image/png`` and ``image/jpeg`` both decode and encode. The interface is the elegant part — a downscale algorithm written against ``image.Image`` does not care whether the source was PNG or JPEG, and the thumbnail route can re-encode in whatever format the bytes decoded as. This guide pulls NO imaging dependency for the downscale itself: a thumbnail is box sampling — every destination pixel is the average of the source box it covers — and a hand-rolled implementation is thirty honest lines that teach more than a library call hides. Alpha is the subtlety that separates a correct implementation from a plausible one: transparent pixels must carry NO color weight, or a logo with transparent edges darkens as it shrinks."),
			code("image-decode-go", "go",
				"Bounded decode: the header is checked against the pixel budget before image.Decode allocates anything.",
				"server/internal/domain/files/thumbnail.go",
				"func DecodeBounded(r io.ReadSeeker) (image.Image, string, error) {\n"+
					"	config, format, err := image.DecodeConfig(bufio.NewReader(r))\n"+
					"	if err != nil {\n"+
					"		return nil, \"\", fmt.Errorf(\"files: read image header: %w\", err)\n"+
					"	}\n"+
					"	if width, height := int64(config.Width), int64(config.Height); width*height > MaxPixels {\n"+
					"		return nil, \"\", fmt.Errorf(\"%w: %d×%d = %d pixels (budget %d)\",\n"+
					"			ErrImageTooLarge, config.Width, config.Height, width*height, MaxPixels)\n"+
					"	}\n"+
					"	if _, err := r.Seek(0, io.SeekStart); err != nil {\n"+
					"		return nil, \"\", fmt.Errorf(\"files: rewind image for decode: %w\", err)\n"+
					"	}\n"+
					"	img, decoded, err := image.Decode(r)\n"+
					"	if err != nil {\n"+
					"		return nil, \"\", fmt.Errorf(\"files: decode %s image: %w\", format, err)\n"+
					"	}\n"+
					"	return img, decoded, nil\n"+
					"}"),
			text("image-bomb",
				"The decode bomb is the image topic's hostile input, and it is a HEADER problem: a few dozen bytes can CLAIM to be a 30000×30000 image, and a naive ``image.Decode`` obligingly allocates gigabytes of pixel memory before discovering the body is truncated. The defense is ordering: ``image.DecodeConfig`` reads only the header (dimensions and format, no pixels), the budget check refuses claims beyond 24 million pixels — about a 6000×4000 photograph, ~96 MB as RGBA, generous for real uploads and fatal for bombs — and only then does the full decode run. The seeker rewind between the two passes is a Go gotcha this guide hit and pinned: ``DecodeConfig`` CONSUMES the header from whatever reader it is handed, so continuing on one shared stream would start the full decode mid-image and die with ``unknown format``. The rewind costs one re-read of a tiny prefix and buys a correct pipeline — the bomb is refused from 33 bytes, which is the whole defense working."),
			code("image-thumbnail-go", "go",
				"The box-sample core: alpha-weighted averaging, so transparent neighbors never darken a pixel.",
				"server/internal/domain/files/thumbnail.go",
				"			var sumR, sumG, sumB, sumA, samples uint64\n"+
					"			for sy := boxTop; sy < boxBottom && sy < bounds.Max.Y; sy++ {\n"+
					"				for sx := boxLeft; sx < boxRight && sx < bounds.Max.X; sx++ {\n"+
					"					c := color.NRGBAModel.Convert(img.At(sx, sy)).(color.NRGBA)\n"+
					"					// Weight color channels by alpha: transparent\n"+
					"					// pixels must not darken what shows through them.\n"+
					"					sumR += uint64(c.R) * uint64(c.A)\n"+
					"					sumG += uint64(c.G) * uint64(c.A)\n"+
					"					sumB += uint64(c.B) * uint64(c.A)\n"+
					"					sumA += uint64(c.A)\n"+
					"					samples++\n"+
					"				}\n"+
					"			}\n"+
					"			// …fully transparent boxes stay transparent; the rest divide…\n"+
					"			dst.SetNRGBA(x, y, color.NRGBA{\n"+
					"				R: uint8(sumR / sumA),\n"+
					"				G: uint8(sumG / sumA),\n"+
					"				B: uint8(sumB / sumA),\n"+
					"				A: uint8(sumA / samples),\n"+
					"			})"),
			text("image-route",
				"The thumbnail route composes the pieces behind one validated knob: ``?max=N`` clamped to 16-1024 (the query-parsed bound this guide's validation topic taught), decode bounded, downscale, re-encode in the source format, serve once. Thumbnails are small by construction, so this is the one place in the topic where holding the artifact in a buffer is the POINT — a bounded ``bytes.Buffer`` beats a streaming multipart response for something measured in kilobytes, and honesty about when to hold memory is as much part of the doctrine as refusing to. The demo's thumbnail beat proves the whole path: a 320×200 fixture decodes, downscales to exactly 64×40 (aspect preserved — the longest edge is the bound), round-trips through BOTH encodings, and the bomb dies at its header while the story watches."),
		),
		// ── 7. Path traversal ───────────────────────────────────────
		section("path-traversal", "Path traversal: the three-lock name jail",
			"A user-supplied filename must never become a filesystem path unguarded. The store's jail is three locks, each catching what the others cannot see.",
			text("traversal-pitch",
				"``../../etc/passwd`` is the topic's oldest attack and it still works because the bug is one innocent-looking line: ``filepath.Join(root, userFilename)``. The defense is not clever parsing — it is an allowlist posture on STRUCTURE, then two re-proofs of the result. The first lock, the sanitizer, asks what a legitimate filename even is: one filesystem component. A name that needs a separator, a ``..``, a NUL byte, or an absolute prefix is refused BEFORE the filesystem is touched — not because every spelling is exploitable on every OS, but because the legitimate spellings a download route serves are plain names and everything else smuggles structure. Absolute paths, both separator families (``/`` and ``\\`` — Windows-hostile names are refused on every platform), dot segments, and NUL bytes all die at the door:"),
			code("traversal-sanitize-go", "go",
				"Lock one: the sanitizer refuses structure by name; lock two: Confine re-proves the string stays under the root.",
				"server/internal/domain/files/files.go",
				"func SanitizeFilename(raw string) (string, error) {\n"+
					"	name := strings.TrimSpace(raw)\n"+
					"	if name == \"\" {\n"+
					"		return \"\", fmt.Errorf(\"%w: %q is empty\", ErrInvalidName, raw)\n"+
					"	}\n"+
					"	if name == \".\" || name == \"..\" {\n"+
					"		return \"\", fmt.Errorf(\"%w: %q is a directory reference\", ErrInvalidName, raw)\n"+
					"	}\n"+
					"	if strings.ContainsAny(name, `/\\`) {\n"+
					"		return \"\", fmt.Errorf(\"%w: %q carries a path separator\", ErrInvalidName, raw)\n"+
					"	}\n"+
					"	// …NUL bytes and any embedded '..' refused the same way…\n"+
					"	return name, nil\n"+
					"}\n"+
					"\n"+
					"func Confine(root, name string) (string, error) {\n"+
					"	cleanRoot := filepath.Clean(root)\n"+
					"	resolved := filepath.Clean(filepath.Join(cleanRoot, name))\n"+
					"	if !strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) {\n"+
					"		return \"\", fmt.Errorf(\"%w: %q escapes the store root\", ErrInvalidName, name)\n"+
					"	}\n"+
					"	return resolved, nil\n"+
					"}"),
			list("traversal-locks", false,
				"SanitizeFilename — the structure gate: separators, dot segments, NUL bytes, and absolute paths are refused by NAME, before any filesystem call.",
				"Confine — the string jail: the joined path is Cleaned and must keep the store root as a strict prefix, catching the encodings that survive eyeball checks.",
				"EvalSymlinks — the filesystem jail: the resolved path is re-proved over the REAL filesystem, because a symlink planted inside the root can point anywhere a string check cannot see.",
			),
			text("traversal-symlink",
				"Why three locks when one seems sufficient: each lock has a blind spot the next one covers. The sanitizer cannot see the FILESYSTEM — a clean-looking name that is a symlink inside the root points wherever its target says, and no string check on the name reveals it. ``Confine`` proves the string, not the disk. So the adapter's third lock resolves symlinks on BOTH ends — the root and the object path — and re-runs the prefix test over the resolved reality; a ``leak.txt`` symlink aimed at ``/tmp/secret`` passes two locks and dies at the third, as the workbench demo proves live. The not-yet-existing case matters too: a fresh upload's target does not resolve, so the adapter resolves its PARENT directory and re-appends the name — new objects get the same jail as old ones:"),
			code("traversal-jail-go", "go",
				"Lock three: the adapter re-proves the prefix over the real filesystem, symlink escapes included.",
				"server/internal/data/files/local.go",
				"func (s *LocalStore) pathFor(name string) (string, error) {\n"+
					"	clean, err := domfiles.SanitizeFilename(name)\n"+
					"	if err != nil {\n"+
					"		return \"\", err\n"+
					"	}\n"+
					"	resolved, err := domfiles.Confine(s.root, clean)\n"+
					"	if err != nil {\n"+
					"		return \"\", err\n"+
					"	}\n"+
					"	realRoot, err := filepath.EvalSymlinks(s.root)\n"+
					"	if err != nil {\n"+
					"		return \"\", fmt.Errorf(\"files: resolve store root: %w\", err)\n"+
					"	}\n"+
					"	real, err := filepath.EvalSymlinks(resolved)\n"+
					"	// …a not-yet-existing target resolves its parent, then re-appends…\n"+
					"	if !strings.HasPrefix(real, realRoot+string(filepath.Separator)) {\n"+
					"		return \"\", fmt.Errorf(\"%w: %q resolves outside the store root\", domfiles.ErrInvalidName, name)\n"+
					"	}\n"+
					"	return real, nil\n"+
					"}"),
			text("traversal-honesty",
				"Two honesty notes complete the section. First, Go's own multipart stack basenames filenames before your code sees them (``Part.FileName()`` strips the directories), so the HTTP route starts from defanged input — the jail still matters because the store's PORT contract must hold for every caller, and because trusting a framework's current sanitization is a compatibility time bomb, not a design. Second, the jail refuses reads that escape but permits writes that land: saving over a jailed name replaces it by ATOMIC RENAME, which is a normal save, not an escape — the object never leaves the root because the rename target was jailed before the temp file was staged. The typed errors carry the verdicts end to end: ``ErrInvalidName`` maps to the 422 envelope at the edge, the sweep's six hostile spellings all refused, and the clean object still serving afterwards is the proof the jail protects without breaking."),
		),
		// ── 8. Terminal companion ───────────────────────────────────
		section("terminal-companion", "Terminal companion: the files curl tour",
			"Upload with -F, download with Range headers, trip the 416, refuse a traversal, watch the workbench story — every command against the running stack on :8100.",
			text("term-files-pitch",
				"With the stack up (server on :8100), this tour exercises the topic's whole edge with curl. ``-F`` builds a real multipart body — the same framing the streaming route parses part by part. ``-H 'Range: …'`` turns curl into a range client; the 206 comes back with its Content-Range, the suffix and the past-the-end 416 with theirs. ``--path-as-is`` stops curl from normalizing ``..`` so the ROUTE's own jail answers. And the workbench story runs the five beats against a fresh private store, exactly as the inline demo renders them:"),
			code("term-upload-download", "bash", "", "",
				"# a real multipart upload — -F streams the file under a \"file\" part\n"+
					"curl -si -F \"file=@internal/domain/files/files.go\" localhost:8100/api/demo/files/upload | head -12\n"+
					"# the 201 body answers with the SNIFFED content type, never the client's claim\n"+
					"\n"+
					"# the whole object back — read the disposition clauses off the wire\n"+
					"curl -si localhost:8100/api/demo/files/download/files.go | head -12\n"+
					"# → Content-Disposition: attachment; filename=\"files.go\"; filename*=UTF-8''files.go\n"+
					"# → Accept-Ranges: bytes   ← the invitation this tour takes up next"),
			code("term-ranges", "bash", "", "",
				"# a mid-file slice: 206, and the body is EXACTLY those bytes\n"+
					"curl -si -H 'Range: bytes=100-163' localhost:8100/api/demo/files/download/files.go | head -14\n"+
					"# → 206 Partial Content · Content-Range: bytes 100-163/… · exact Content-Length\n"+
					"\n"+
					"# the suffix (last 64 bytes) and the honest refusal past the end\n"+
					"curl -si -H 'Range: bytes=-64' localhost:8100/api/demo/files/download/files.go | head -6\n"+
					"curl -si -H 'Range: bytes=9999999-' localhost:8100/api/demo/files/download/files.go | head -8\n"+
					"# → 416 with Content-Range: bytes */… — the census, not an error\n"+
					"\n"+
					"# a malformed range is IGNORED per RFC 7233: the whole 200, never half\n"+
					"curl -s -o /dev/null -w '%{http_code} %{size_download}\\n' -H 'Range: bytes=99-0' \\\n"+
					"  localhost:8100/api/demo/files/download/files.go"),
			code("term-jail-thumb-story", "bash", "", "",
				"# the traversal attempt, un-normalized so the route's own jail answers\n"+
					"curl -si --path-as-is localhost:8100/api/demo/files/download/../../etc/passwd | head -6\n"+
					"# → the mux redirects cleaned paths; the smuggled-separator refusal is the 422:\n"+
					"curl -si 'localhost:8100/api/demo/files/download/smuggled%2F..%2Fetc' | head -6\n"+
					"\n"+
					"# the thumbnail guard: a .go file is not an image — typed 422, not a crash\n"+
					"curl -s 'localhost:8100/api/demo/files/thumb/files.go?max=64' | python3 -m json.tool | head -8\n"+
					"\n"+
					"# the workbench story: five beats, derived proofs, one reasoning paragraph\n"+
					"curl -s localhost:8100/api/demo/files/workbench | python3 -m json.tool | head -40\n"+
					"# the topic on the wire — order 26 opens the media-realtime group\n"+
					"curl -s localhost:8100/api/content | python3 -c 'import json,sys; d=json.load(sys.stdin); print([t[\"id\"] for t in d[\"topics\"] if t[\"group\"]==\"media-realtime\"])'"),
			text("term-files-coda",
				"Read the tour as the topic's five proofs, re-runnable at will: the upload beat's allocation honesty (the 201 answers with metadata derived from the stored bytes, and the workbench story puts the number on it), the 206's exactness (Content-Range, Content-Length, and a body you can diff against a cut of the file), the 416's census (a refusal that still tells the truth about size), the jail's three locks (the smuggled separator refused at the edge, the symlink caught inside the story), and the thumbnail's budget refusing a claim without allocating a pixel. Files are the largest cargo a backend ever moves, and the doctrine never changes: stream both ways, bound everything, derive metadata from evidence, jail every name — and when the bytes are images, refuse the ones that arrive as a header claiming gigabytes. The realtime topic next moves to cargo that never stops arriving."),
		),
	}

	if buildErr != nil {
		return Topic{}, buildErr
	}
	return NewTopic(
		"files-media",
		"Files and media from Go",
		"Uploads are streams, not slurps — bounded by io.LimitReader, staged through temp-file-plus-rename, parsed with r.MultipartReader — a streaming honesty carried into downloads, range requests, backpressure, and image processing.",
		"media-realtime",
		26,
		sections,
	)
}
