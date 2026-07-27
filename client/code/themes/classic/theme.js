function chatUserInterface (userOptions) { //5/2/26 by Claude + DW -- classic theme (3-col), renamed from twitter 7/2/26
			console.log ("chatUserInterface (classic)");

			const themesVersion = "0.5.339"; //bump on every theme edit -- the first Return after pasting text containing a link now breaks the paragraph at the cursor, not at the link, 7/23/26 by CC

			var options = {
				whereToAppend: undefined,
				placeholderSignedIn: "What's happening?",
				placeholderSignedOut: "Sign in to post",
				placeholderReply: "Post your reply",
				ctRecentItems: 100,
				draftKey: "rssNetworkComposerDraft",
				titleKey: "rssNetworkComposerTitle",
				writingModeKey: "rssNetworkWritingMode",
				iconBar: undefined,
				itemIconBar: undefined, //5/22/26 by Claude -- array of icon objects rendered at the bottom of each timeline item
				itemSelectedCallback: undefined, //6/1/26 by Claude -- called with the item when the cursor moves to a different item
				itemSeenCallback: undefined, //7/2/26 by CC -- #129: called with the item when a post has actually been visible on screen; the app uses it to advance the read bookmark
				idLastMessageRead: undefined, //7/2/26 by CC -- the read bookmark as of arrival; nothing visible uses it since #133, kept for a future UI
				timelineItemMenu: undefined, //6/10/26 by Claude -- array of menu entry objects for each timeline item's three-dot menu
				editorMenu: undefined, //6/10/26 by Claude -- array of menu entry objects for the edit window's three-dot menu
				avatarClickedCallback: undefined, //6/15/26 by Claude -- called with (screenname, ev) when any avatar is clicked
				storyGuid: undefined, //6/19/26 by Claude -- when set, render the one post with this guid as a standalone story page instead of the timeline; the app parses the URL, the theme never does
				profileName: undefined, //6/26/26 by CC -- #79: when set, open that user's profile page instead of the timeline; the app parses ?name= from the URL, the theme never does
				};
			mergeOptions (userOptions, options);

			const divChat = $('<div class="divChat"></div>');
			const divChatLeft = $('<div class="divChatLeft"></div>');
			const divChatCenter = $('<div class="divChatCenter"></div>');
			const divChatRight = $('<div class="divChatRight"></div>');

			const divBrand = $('<div class="divBrand">rss.network</div>');
			const divNav = $('<div class="divNav"></div>');
			const divNavHome = $('<div class="divNavItem divNavActive">Home</div>');
			const divNavCompose = $('<div class="divNavItem">Compose</div>');
			const divUserProfile = $('<div class="divUserProfile"></div>');

			const divCenterHeader = $('<div class="divCenterHeader"></div>');
			const divComposer = $('<div class="divComposer"></div>');
			const divComposerAvatar = $('<div class="divComposerAvatar"></div>');
			const divComposerBody = $('<div class="divComposerBody"></div>');
			const divComposerToolbar = $('<div class="divComposerToolbar"></div>');
			const buttonBold = $('<button class="buttonToolbar" title="Bold"><i class="fas fa-bold"></i></button>');
			const buttonItalic = $('<button class="buttonToolbar" title="Italic"><i class="fas fa-italic"></i></button>');
			const buttonLink = $('<button class="buttonToolbar" title="Link"><i class="fas fa-link"></i></button>');
			const buttonBulletList = $('<button class="buttonToolbar" title="Bulleted list"><i class="fas fa-list-ul"></i></button>');
			const buttonNumberList = $('<button class="buttonToolbar" title="Numbered list"><i class="fas fa-list-ol"></i></button>');
			const divComposerTitle = $('<div class="divComposerTitle"></div>');
			if (options.titleTooltip !== undefined) { //6/7/26 by Claude -- #35, Bootstrap tooltip below the title box, not native title=
				addToolTip (divComposerTitle, options.titleTooltip, "bottom");
				}
			const divContextMenu = $('<div class="divContextMenu"></div>');
			const buttonContextMenu = $('<button class="buttonToolbar buttonContextMenu" title="More"><i class="fas fa-ellipsis-v"></i></button>');
			const divContextMenuPopup = $('<div class="divContextMenuPopup"></div>');
			const inputComposer = $('<div class="inputComposer" contenteditable="true"></div>');
			const buttonPost = $('<button class="buttonPost">Post</button>');
			const divTimeline = $('<div class="divTimeline"></div>');
			const divStory = $('<div class="divStory"></div>'); //6/20/26 by Claude -- own area for the in-place story page, so the timeline sits untouched underneath
			const divProfile = $('<div class="divProfile"></div>'); //6/26/26 by CC -- #79: own area for the in-place profile page, same idea as divStory
			const divScanner = $('<div class="divScanner"></div>'); //6/27/26 by CC -- #104: own area for the Message Scanner, same idea as divStory
			const divMapOutline = $('<div class="divMapOutline"></div>'); //7/9/26 by CC -- the left-panel map: an outline of the posts the middle column is showing, one line per post, wedges and all

			const divFeedCard = $('<div class="divCard divFeedCard"></div>');
			const divUsersCard = $('<div class="divCard divUsersCard"></div>');

			const itemsById = {};
			const itemsByGuid = {}; //look up displayed item by guid for updateItem
			var savedTimelineScroll = 0; //6/20/26 by Claude -- where the timeline was scrolled when we left it for a story, restored on the way back
			var hoistStack = new Array (); //6/27/26 by CC -- #104 hoist/dehoist (MORE/Drummer lineage): ids of the posts hoisted into, outermost first; the last is the current temporary root. Empty == not hoisted, normal timeline showing
			var hoistScrollStack = new Array (); //6/27/26 by CC -- #104: parallel to hoistStack -- the window scroll position of the view you left when you hoisted, restored when you dehoist back to it
			var flTimelineLoaded = false; //6/29/26 by CC -- #118: a cold-load into a story/profile never loads the timeline; load it the first time we return to it

			var divSelectedThread = undefined;
			var currentTitle = undefined;
			var replyTargetItem = undefined;
			var editTargetItem = undefined; //5/21/26 by Claude
			var savedTitleBeforeEdit; //5/21/26 by Claude -- snapshot of currentTitle during an edit session
			var editOriginalTitle, editOriginalBody, editOriginalMd; //5/24/26 by Claude -- baseline for the Update-button enable-on-change test; editOriginalMd added 7/5/26 by CC for #107
			var pendingReplyMap = {}; //guid -> inReplyTo, cleared when socket delivers the item
			var savedMessage; //overrides draft-state text when set (e.g. "PUBLISHING")
			var flDraftChanged = false;
			var draftSaveTimeout, theSpinner; //assigned by draft/publish flow
			var currentEditorMode = appPrefs.defaultEditorMode || "wizzy";
			var ctPendingImageUploads = 0, idNextImageUpload = 1; //7/22/26 by CC -- #188: publishing waits while images upload

			const divReplyOverlay = $('<div class="divReplyOverlay"></div>');
			const divReplyModal = $('<div class="divReplyModal"></div>');
			const divReplyModalTop = $('<div class="divReplyModalTop"></div>');
			const divReplyParent = $('<div class="divReplyParent"></div>');
			const divReplyParentAvatar = $('<div class="divReplyParentAvatar"></div>');
			const divReplyParentBody = $('<div class="divReplyParentBody"></div>');
			const divReplyParentAuthor = $('<div class="divReplyParentAuthor"></div>');
			const divReplyParentTitle = $('<div class="divReplyParentTitle"></div>');
			const divReplyParentText = $('<div class="divReplyParentText"></div>');
			const divReplyParentSummary = $('<div class="divReplyParentSummary"><span class="spanReplyWedge">▸</span><span class="spanReplyWordCount"></span></div>'); //7/20/26 by CC -- #155: the collapsed stand-in for the post being replied to; the wedge is a text character, not an icon-font glyph //black right-pointing small triangle
			const divReplyParentDivider = $('<div class="divReplyDividerLine"></div>');
			const divReplyComposerRow = $('<div class="divReplyComposerRow"></div>');
			const divModalToolbar = $('<div class="divModalToolbar"></div>');
			const buttonModalBold = $('<button class="buttonModalTool" title="Bold"><i class="fas fa-bold"></i></button>');
			const buttonModalItalic = $('<button class="buttonModalTool" title="Italic"><i class="fas fa-italic"></i></button>');
			const buttonModalLink = $('<button class="buttonModalTool" title="Link"><i class="fas fa-link"></i></button>');
			const buttonModalBulletList = $('<button class="buttonModalTool" title="Bulleted list"><i class="fas fa-list-ul"></i></button>');
			const buttonModalNumberList = $('<button class="buttonModalTool" title="Numbered list"><i class="fas fa-list-ol"></i></button>');
			const buttonModalMarkdown = $('<button class="buttonModalTool" title="Toggle markdown mode"><i class="fab fa-markdown"></i></button>');
			const divModalBottom = $('<div class="divModalBottom"></div>');
			const divModalStatus = $('<div class="divModalStatus"></div>');
			const divModalWhen = $('<div class="divModalWhen"></div>');
			const divModalCharCount = $('<div class="divModalCharCount"></div>');
			const buttonReplyCancel = $('<button class="buttonReplyCancel btn btn-default">Cancel</button>');
			const buttonReplyPost = $('<button class="buttonReplyPost btn btn-primary">Reply</button>');
			const divReplyComposerAvatar = $('<div class="divReplyComposerAvatar"></div>');
			const inputReplyComposer = $('<div class="inputReplyComposer" contenteditable="true"></div>').attr ('data-placeholder', 'Write your reply');
			const textareaMarkdown = $('<textarea class="textareaMarkdown"></textarea>'); //7/5/26 by CC -- #107: markdown mode edits in a real text box, wordland-style -- angle brackets and markdown syntax are just characters here
			var mdSourceText = ""; //7/5/26 by CC -- #107: the markdown text is the one permanent document (the wordland model); the wizzy view is a rendering of it; the input handlers keep it current, so flipping modes never converts a conversion
			document.execCommand ("defaultParagraphSeparator", false, "p"); //5/26/26 by Claude -- Enter produces <p> not <div>, so paragraphs survive feedland's HTML strip
			var flReplyParentShowing = false; //7/20/26 by CC -- #155: the wedge shows and hides the post being replied to; it starts hidden every time
			var flReplyParentHasTitle = false; //assigned by openReplyModal
			function setReplyParentVisible (flShow) {
				flReplyParentShowing = flShow;
				if (flShow) {
					if (flReplyParentHasTitle) {
						divReplyParentTitle.show ();
						}
					divReplyParentText.show ();
					divReplyParentSummary.addClass ("flExpanded");
					}
				else {
					divReplyParentTitle.hide ();
					divReplyParentText.hide ();
					divReplyParentSummary.removeClass ("flExpanded");
					}
				}
			divReplyParentSummary.on ("click", function () {
				setReplyParentVisible (!flReplyParentShowing);
				});
			function getWordCountText (htmltext) { //7/20/26 by CC -- #155
				const plaintext = $("<div></div>").html (htmltext).text ();
				var ctWords = 0;
				plaintext.split (/\s+/).forEach (function (word) {
					if (word.length > 0) {
						ctWords++;
						}
					});
				return (ctWords + ((ctWords === 1) ? " word." : " words."));
				}

			function selectThread (divThread, flScrollIntoView) {
				const flChanged = (divSelectedThread === undefined) || (divSelectedThread [0] !== divThread [0]);
				if (divSelectedThread !== undefined) {
					divSelectedThread.removeClass ("flSelected");
					}
				divSelectedThread = divThread;
				divThread.addClass ("flSelected");
				if (flScrollIntoView === true) { //only keyboard nav scrolls; a click is already on-screen, scrolling would nudge the timeline
					scrollThreadIntoView (divThread);
					}
				if (flChanged && (typeof options.itemSelectedCallback === "function")) { //6/1/26 by Claude -- notify the shell the cursor moved to a different item
					const theItem = getItemForThread (divThread);
					if (theItem !== undefined) {
						options.itemSelectedCallback (theItem);
						}
					}
				syncMapSelection (); //7/9/26 by CC -- the map's you-are-here highlight follows the cursor
				}
			function deselectThread () {
				if (divSelectedThread !== undefined) {
					divSelectedThread.removeClass ("flSelected");
					divSelectedThread = undefined;
					syncMapSelection (); //7/9/26 by CC
					}
				}
			function scrollThreadIntoView (divThread) { //6/10/26 by Claude -- #57: keep the navigated-to item clear of the fixed menu at top and the window bottom; scrollIntoView's "nearest" ignores the menu and leaves items tucked under it
				const menuBarHeight = 65; //matches the body padding-top that clears the fixed top menu bar
				const rect = divThread [0].getBoundingClientRect ();
				if (rect.top < menuBarHeight) {
					window.scrollBy (0, rect.top - menuBarHeight);
					}
				else {
					if (rect.bottom > window.innerHeight) {
						window.scrollBy (0, rect.bottom - window.innerHeight);
						}
					}
				}
			function getItemForThread (divThread) { //6/1/26 by Claude -- resolve the chatItem behind a thread element. 6/26 by CC -- read the item straight off the thread, so replies and story/profile posts resolve too, not just timeline posts in itemsById
				if (divThread === undefined) {
					return (undefined);
					}
				return (divThread.data ("item"));
				}
			function getActiveMapContainer () { //7/9/26 by CC -- the map mirrors whichever area the middle column is showing
				var theContainer = divTimeline;
				if (divChat.hasClass ("storyPage")) {
					theContainer = divStory;
					}
				else {
					if (divChat.hasClass ("profilePage")) {
						theContainer = divProfile;
						}
					else {
						if (divChat.hasClass ("scannerPage")) {
							theContainer = divScanner;
							}
						}
					}
				return (theContainer);
				}
			function getMapLineText (theItem) { //7/9/26 by CC -- what a post's line in the map says: its title when it has one, otherwise the words of the post with the markup stripped
				var theText = "";
				if ((theItem.title !== undefined) && (theItem.title.length > 0)) {
					theText = theItem.title;
					}
				else {
					theText = $("<div></div>").html (theItem.description || "").text ();
					}
				return (theText);
				}
			function renderMap () { //7/9/26 by CC -- rebuild the map from what's rendered in the middle column; it's a map of what you're looking at, not of everything that exists
				function renderMapLevel (theContainer, level) {
					theContainer.children (".divThread").each (function () {
						const divPostThread = $(this);
						const theItem = divPostThread.data ("item");
						if (theItem !== undefined) {
							const divMapLine = $('<div class="divMapLine"></div>');
							divMapLine.css ("padding-left", (level * 14) + "px");

							const divMapWedge = $('<div class="divMapWedge"><i class="fas fa-caret-right"></i></div>');
							const wedgeUI = divPostThread.data ("wedgeUI");
							if (wedgeUI !== undefined) { //the map wedge is a twin of the post's wedge -- same shade, same click
								if (wedgeUI.divWedgeReplies.hasClass ("wedgeDark")) {
									divMapWedge.addClass ("wedgeDark");
									}
								if (wedgeUI.divWedgeReplies.hasClass ("wedgeClickable")) {
									divMapWedge.addClass ("wedgeClickable");
									}
								divMapWedge.on ("click", function (ev) {
									ev.stopPropagation ();
									wedgeUI.toggleReplies ();
									});
								}

							const divMapLineText = $('<div class="divMapLineText"></div>');
							const spanMapAuthor = $('<span class="spanMapAuthor"></span>').text (theItem.author || theItem.screenname || "?");
							const spanMapText = $('<span class="spanMapText"></span>').text (getMapLineText (theItem));
							divMapLineText.append (spanMapAuthor);
							divMapLineText.append (spanMapText);
							addToolTip (divMapLineText, getMapLineText (theItem).substring (0, 400), "right"); //the blogroll move: the line stays short, the hover carries the words

							divMapLine.append (divMapWedge);
							divMapLine.append (divMapLineText);
							if ((divSelectedThread !== undefined) && (divPostThread [0] === divSelectedThread [0])) {
								divMapLine.addClass ("flSelected"); //you are here
								}
							divMapLine.data ("mapThread", divPostThread);
							divMapLine.on ("click", function () {
								selectThread (divPostThread, true);
								});
							divMapOutline.append (divMapLine);

							renderMapLevel (divPostThread.children (".divReplies"), level + 1);
							}
						});
					}
				divMapOutline.empty ();
				renderMapLevel (getActiveMapContainer (), 0);
				}
			var mapRefreshTimer; //assigned by scheduleRenderMap
			function scheduleRenderMap () { //7/9/26 by CC -- posts render in bursts; coalesce them into one map rebuild
				if (mapRefreshTimer !== undefined) {
					clearTimeout (mapRefreshTimer);
					}
				mapRefreshTimer = setTimeout (function () {
					mapRefreshTimer = undefined;
					renderMap ();
					}, 100);
				}
			function syncMapSelection () { //7/9/26 by CC -- move the you-are-here highlight when the cursor moves, without rebuilding the map
				divMapOutline.children (".divMapLine").each (function () {
					const divMapLine = $(this);
					const divPostThread = divMapLine.data ("mapThread");
					if ((divSelectedThread !== undefined) && (divPostThread !== undefined) && (divPostThread [0] === divSelectedThread [0])) {
						divMapLine.addClass ("flSelected");
						}
					else {
						divMapLine.removeClass ("flSelected");
						}
					});
				}
			function fillMenuPopup (divPopup, theMenu, decorateEvent) { //6/10/26 by Claude -- build a popup menu's entries from a menu structure the app passes in
				theMenu.forEach (function (menuDef) {
					if (menuDef.flDivider === true) { //a divider line, not a clickable entry
						divPopup.append ($('<div class="divContextMenuDivider"></div>'));
						}
					else {
						const divMenuEntry = $('<div class="divContextMenuItem"></div>').text (menuDef.display);
						divMenuEntry.data ("menuDef", menuDef); //per-second refresh reads enabled from here
						if (menuDef.tooltip !== undefined) {
							divMenuEntry.attr ("title", menuDef.tooltip);
							}
						divMenuEntry.on ("click", function (ev) {
							if (menuDef.enabled !== true) { //disabled entries ignore clicks
								return;
								}
							divPopup.removeClass ("flOpen");
							if (decorateEvent !== undefined) {
								decorateEvent (ev);
								}
							menuDef.click (ev);
							});
						divPopup.append (divMenuEntry);
						}
					});
				}
			function refreshIconsEnabled () { //6/1/26 by Claude -- mirror each icon's enabled value into the DOM, once a second
				divChat.find (".divInsideItemIcon, .divIconsContainer .divIcon").each (function () {
					const divIcon = $(this);
					const iconDef = divIcon.data ("iconDef");
					if (iconDef === undefined) {
						return;
						}
					if (iconDef.enabled === true) {
						divIcon.addClass ("enabled");
						}
					else {
						divIcon.removeClass ("enabled");
						}
					});
				divChat.find (".divContextMenuItem").add (divReplyOverlay.find (".divContextMenuItem")).each (function () { //6/10/26 by Claude -- same mirror for menu entries; the edit window lives outside the chat container
					const divEntry = $(this);
					const menuDef = divEntry.data ("menuDef");
					if (menuDef === undefined) {
						return;
						}
					if (menuDef.enabled === true) {
						divEntry.removeClass ("flDisabled");
						}
					else {
						divEntry.addClass ("flDisabled");
						}
					});
				}
			function lockBodyScroll () { //pad the body by the scrollbar width before hiding it, so the centered page doesn't jump right when a modal opens
				const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
				$("body").css ("padding-right", scrollbarWidth + "px");
				$("body").css ("overflow", "hidden");
				}
			function unlockBodyScroll () {
				$("body").css ("overflow", "");
				$("body").css ("padding-right", "");
				}

			function getTimeString (theDate) {
				return (theDate.toLocaleTimeString ([], {hour: "2-digit", minute: "2-digit"}));
				}

			function populateAvatar (divAvatarElement, imageUrl, initial) {
				divAvatarElement.empty ();
				if (imageUrl) {
					divAvatarElement.append ($('<img class="imgAvatar" />').attr ("src", imageUrl));
					}
				else {
					divAvatarElement.text (initial);
					}
				}
			function wireAvatarClick (divAvatarElement, theScreenname) { //6/15/26 by Claude -- any avatar is clickable, fires the app's avatarClickedCallback
				divAvatarElement.css ("cursor", "pointer");
				divAvatarElement.off ("click.avatar").on ("click.avatar", function (ev) {
					if (options.avatarClickedCallback !== undefined) {
						ev.stopPropagation (); //don't also select/expand the post under the avatar
						options.avatarClickedCallback (theScreenname, ev);
						}
					});
				}

			function saveDraft (flEditorOpen) { //6/21/26 by CC -- save the editor in any mode (new, reply, edit) so a reload never loses what you wrote. 6/25 #87: flEditorOpen (default true) records whether the editor was open or being dismissed, so a reload only auto-reopens it if it was open
				var theText;
				if (currentEditorMode === "wizzy") {
					theText = inputReplyComposer.html ();
					}
				else {
					theText = textareaMarkdown.val (); //7/5/26 by CC -- #107: markdown mode's text lives in the markdown text box now
					}
				if (getEditorText ().trim ().length === 0) { //6/25/26 by CC -- #87: nothing worth keeping; don't leave an empty draft behind for the Show editor command to reopen
					clearDraft ();
					return;
					}
				var theDraft = {text: theText, mode: currentEditorMode, title: currentTitle, context: "new"};
				if (editTargetItem !== undefined) {
					theDraft.context = "edit";
					theDraft.targetGuid = editTargetItem.guid;
					}
				else {
					if (replyTargetItem !== undefined) {
						theDraft.context = "reply";
						theDraft.targetGuid = replyTargetItem.guid;
						}
					}
				theDraft.flOpen = (flEditorOpen !== false); //6/25/26 by CC -- #87: remember whether the editor was open or dismissed, so a reload only auto-reopens it if it was open
				appPrefs.savedUserDraft = theDraft;
				prefsChanged ();
				}

			function clearDraft () {
				appPrefs.savedUserDraft = undefined;
				prefsChanged ();
				}

			function saveTitle () {
				if (currentTitle !== undefined) {
					localStorage.setItem (options.titleKey, currentTitle);
					}
				else {
					localStorage.removeItem (options.titleKey);
					}
				}

			function editNewItem () {
				currentTitle = undefined;
				saveTitle ();
				refreshComposerTitle ();
				clearDraft ();
				openEditWindow ();
				}

			function getEditorText () { //7/5/26 by CC -- #107: the editor's plain text, from whichever surface is active -- for emptiness tests and the char count
				if (currentEditorMode === "markdown") {
					return (textareaMarkdown.val ());
					}
				else {
					return (inputReplyComposer.text ());
					}
				}
			function focusEditor () { //7/5/26 by CC -- #107: focus whichever surface is showing
				if (currentEditorMode === "markdown") {
					textareaMarkdown.focus ();
					}
				else {
					inputReplyComposer.focus ();
					}
				}
			function setEditorMode (theMode) { //7/5/26 by CC -- #107: the mode picks which surface shows -- the wizzy view or the markdown text box; the markdown box always shows the stored document. 7/6/26 by CC -- #159: no more self-growing (wordland's fixHeight was for a page that scrolls; our edit window is height-capped) -- the box fills the available space and scrolls, like the wizzy surface
				currentEditorMode = theMode;
				if (theMode === "markdown") {
					buttonModalMarkdown.addClass ("spMarkdownOn");
					inputReplyComposer.addClass ("flMarkdownMode");
					textareaMarkdown.val (mdSourceText);
					inputReplyComposer.hide ();
					textareaMarkdown.show ();
					}
				else {
					buttonModalMarkdown.removeClass ("spMarkdownOn");
					inputReplyComposer.removeClass ("flMarkdownMode");
					textareaMarkdown.hide ();
					inputReplyComposer.show ();
					}
				}

			function toggleMarkdownMode () { //7/5/26 by CC -- #107: rebuilt on the wordland model. To markdown: show the stored markdown, character for character -- no conversion. To wizzy: render the markdown. Flipping without editing never changes the document
				if (currentEditorMode === "wizzy") {
					setEditorMode ("markdown"); //fills the text box from mdSourceText, which the wizzy input handler kept current
					}
				else {
					mdSourceText = textareaMarkdown.val ();
					setEditorMode ("wizzy");
					if (mdSourceText.trim ().length > 0) {
						inputReplyComposer.html (getHtmlFromMarkdown (mdSourceText));
						inputReplyComposer.addClass ("flUserTouched");
						}
					else {
						inputReplyComposer.html ("");
						inputReplyComposer.removeClass ("flUserTouched");
						}
					}
				focusEditor ();
				updateModalCharCount ();
				if (editTargetItem !== undefined) { //keep the publish/update button's enable current in the new mode
					refreshUpdateDisabled ();
					}
				else {
					buttonReplyPost.prop ("disabled", getEditorText ().trim ().length === 0);
					}
				scheduleDraftSave (); //the draft remembers the mode, so a flip is worth saving
				}

			function wrapSelectionMarkdown (leftMarker, rightMarker) { //7/5/26 by CC -- #107: rewritten for the markdown text box -- splice the markers around the selected characters
				textareaMarkdown.focus ();
				const theTextarea = textareaMarkdown [0];
				const startPos = theTextarea.selectionStart;
				const endPos = theTextarea.selectionEnd;
				const theText = theTextarea.value;
				const selectedText = theText.substring (startPos, endPos);
				const inserted = leftMarker + selectedText + rightMarker;
				theTextarea.value = theText.substring (0, startPos) + inserted + theText.substring (endPos);
				const caretPos = startPos + inserted.length;
				theTextarea.setSelectionRange (caretPos, caretPos);
				textareaMarkdown.trigger ("input");
				}

			var flItemsLoaded = false; //6/21/26 by CC -- set once the timeline's items are in, so a reply/edit draft can find its target post

			function restoreDraftBody (theDraft) { //6/21/26 by CC -- put the saved text back into the editor after the modal is opened. 7/5/26 by CC -- #107: a markdown draft restores into the markdown text box; a wizzy draft restores as html and seeds the markdown document from it
				const theMode = theDraft.mode || appPrefs.defaultEditorMode || "wizzy";
				if (theMode === "markdown") {
					mdSourceText = theDraft.text;
					setEditorMode ("markdown"); //shows the text box, filled from mdSourceText
					}
				else {
					setEditorMode ("wizzy");
					inputReplyComposer.html (theDraft.text);
					mdSourceText = getMarkdownFromHtml (theDraft.text);
					}
				inputReplyComposer.addClass ("flUserTouched");
				updateModalCharCount ();
				}
			function restoreContextDraft (theDraft) { //6/21/26 by CC -- reopen a reply or edit draft against its target post, or keep the text if that post is gone
				const entry = itemsByGuid [theDraft.targetGuid];
				if (theDraft.context === "edit") {
					if (entry !== undefined) {
						openEditModal (entry.item);
						restoreDraftBody (theDraft);
						currentTitle = theDraft.title;
						refreshComposerTitle ();
						refreshUpdateDisabled ();
						}
					else { //the post being edited is gone -- keep the text, but don't let it publish as a new post
						openEditWindow (theDraft);
						buttonReplyPost.prop ("disabled", true);
						}
					}
				else {
					if (entry !== undefined) {
						openReplyModal (entry.item);
						restoreDraftBody (theDraft);
						buttonReplyPost.prop ("disabled", getEditorText ().trim ().length === 0); //7/5/26 by CC -- #107: mode-aware
						}
					else { //the post being replied to is gone -- keep the text, don't silently publish it as a new post
						openEditWindow (theDraft);
						buttonReplyPost.prop ("disabled", true);
						}
					}
				}
			function loadDraft () { //6/21/26 by CC -- new-post drafts restore now; reply/edit drafts wait until their target post is loaded
				const theDraft = appPrefs.savedUserDraft;
				if (theDraft !== undefined && typeof theDraft === "object" && theDraft.text) {
					if ((theDraft.context === "reply") || (theDraft.context === "edit")) {
						if (flItemsLoaded === true) {
							restoreContextDraft (theDraft);
							}
						}
					else {
						openEditWindow (theDraft);
						}
					}
				}
			function restorePendingContextDraft () { //6/21/26 by CC -- called once items are loaded; reopens a reply/edit draft that was waiting for its target
				flItemsLoaded = true;
				const theDraft = appPrefs.savedUserDraft;
				if (theDraft !== undefined && typeof theDraft === "object" && theDraft.text && theDraft.flOpen === true && ((theDraft.context === "reply") || (theDraft.context === "edit"))) {
					restoreContextDraft (theDraft);
					}
				}
			function showEditor () { //6/25/26 by CC -- #87: reopen the editor with the text the user was last working on; returns undefined on success, or an error object if there's nothing in progress to restore (e.g. a fresh launch). The Show editor menu command calls this.
				const theDraft = appPrefs.savedUserDraft;
				if (theDraft !== undefined && typeof theDraft === "object" && theDraft.text) {
					loadDraft ();
					theDraft.flOpen = true; //6/25/26 by CC -- #87: the editor is open again; remember it so an accidental reload restores it
					prefsChanged ();
					return (undefined);
					}
				else {
					const message = "Can't show the editor because you haven't started writing anything yet.";
					return ({message});
					}
				}

			function applyPrefs () { //called from code.js after server prefs are merged into appPrefs
				const theDraft = appPrefs.savedUserDraft; //6/25/26 by CC -- #87: on reload, only auto-reopen the editor if it was open (not deliberately dismissed) when we left
				if (theDraft !== undefined && typeof theDraft === "object" && theDraft.text && theDraft.flOpen === true) {
					loadDraft ();
					}
				}

			function spinningImage (userOptions) {
				var opts = {whereToPrepend: divReplyModal};
				mergeOptions (userOptions, opts);
				const spinContainer = $('<div class="spinner"></div>');
				spinContainer.append ('<svg fill="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><g><circle cx="12" cy="2.5" r="1.5" opacity=".14"></circle><circle cx="16.75" cy="3.77" r="1.5" opacity=".29"></circle><circle cx="20.23" cy="7.25" r="1.5" opacity=".43"></circle><circle cx="21.50" cy="12.00" r="1.5" opacity=".57"></circle><circle cx="20.23" cy="16.75" r="1.5" opacity=".71"></circle><circle cx="16.75" cy="20.23" r="1.5" opacity=".86"></circle><circle cx="12" cy="21.5" r="1.5"></circle><animateTransform attributeName="transform" type="rotate" calcMode="discrete" dur="0.75s" values="0 12 12;30 12 12;60 12 12;90 12 12;120 12 12;150 12 12;180 12 12;210 12 12;240 12 12;270 12 12;300 12 12;330 12 12;360 12 12" repeatCount="indefinite"></animateTransform></g></svg>');
				opts.whereToPrepend.prepend (spinContainer);
				this.stop = function () {
					spinContainer.remove ();
					};
				}
			function updateModalStatus () {
				var theText;
				if (savedMessage !== undefined) {
					theText = savedMessage;
					}
				else {
					theText = flDraftChanged ? "NOT SAVED" : "SAVED";
					}
				divModalStatus.text (theText);
				}
			function showModalWhen (theDate) {
				divModalWhen.empty ();
				divModalWhen.append (getUpdateableTime (theDate, ""));
				divModalWhen.show ();
				}

			function updateModalCharCount () {
				const text = getEditorText (); //7/5/26 by CC -- #107: count whichever surface is active
				if (appPrefs.flWordCount) {
					const words = text.trim () === "" ? 0 : text.trim ().split (/\s+/).length;
					divModalCharCount.text (words + (words === 1 ? " word" : " words"));
					}
				else {
					const chars = text.length;
					divModalCharCount.text (chars + (chars === 1 ? " character" : " characters"));
					}
				}

			function openReplyModal (item) {
				const author = item.author || "?";
				const initial = author.charAt (0).toUpperCase ();
				populateAvatar (divReplyParentAvatar, item.imageUrl || appConsts.urlDefaultImage, initial);
				wireAvatarClick (divReplyParentAvatar, item.screenname);
				divReplyParentAuthor.text (author);
				const theParentTitle = item.title;
				if (theParentTitle === undefined || theParentTitle.length === 0) { //7/20/26 by CC -- #155: setReplyParentVisible owns show/hide now; here we only remember whether there's a title to show
					flReplyParentHasTitle = false;
					divReplyParentTitle.text ("");
					}
				else {
					flReplyParentHasTitle = true;
					divReplyParentTitle.text (theParentTitle);
					}
				divReplyParentText.html (item.description);
				const myScreenname = globals.myRssNetwork.getScreenname () || "?";
				const myInitial = myScreenname.charAt (0).toUpperCase ();
				populateAvatar (divReplyComposerAvatar, appPrefs.myAvatarImageUrl || appConsts.urlDefaultImage, myInitial);
				wireAvatarClick (divReplyComposerAvatar, myScreenname);
				mdSourceText = ""; //7/5/26 by CC -- #107: a fresh reply starts with an empty document
				setEditorMode (appPrefs.defaultEditorMode || "wizzy");
				inputReplyComposer.html ("").removeClass ("flUserTouched").attr ("data-placeholder", options.placeholderReply);
				textareaMarkdown.attr ("placeholder", options.placeholderReply);
				savedMessage = undefined;
				flDraftChanged = false;
				updateModalStatus ();
				showModalWhen (new Date ());
				updateModalCharCount ();
				buttonReplyPost.text ("Reply");
				buttonReplyPost.prop ("disabled", true);
				divReplyModal.removeClass ("flNoParent");
				divReplyParentText.addClass ("flExpanded"); //7/20/26 by CC -- #155: when the wedge opens the post, all of it shows
				divReplyParentSummary.find (".spanReplyWordCount").text (getWordCountText (item.description));
				divReplyParentSummary.show ();
				divReplyParent.show (); //avatar and author name stay visible
				setReplyParentVisible (false); //title and text start folded every time
				divReplyParentDivider.show ();
				replyTargetItem = item;
				lockBodyScroll ();
				divReplyOverlay.fadeIn (300);
				focusEditor ();
				}
			function openEditWindow (savedDraft) {
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				const myScreenname = globals.myRssNetwork.getScreenname () || "?";
				const myInitial = myScreenname.charAt (0).toUpperCase ();
				populateAvatar (divReplyComposerAvatar, appPrefs.myAvatarImageUrl || appConsts.urlDefaultImage, myInitial);
				wireAvatarClick (divReplyComposerAvatar, myScreenname);
				inputReplyComposer.removeClass ("flUserTouched").attr ("data-placeholder", options.placeholderSignedIn);
				textareaMarkdown.attr ("placeholder", options.placeholderSignedIn);
				if (savedDraft !== undefined && savedDraft.text) { //7/5/26 by CC -- #107: restore per mode -- a markdown draft into the text box, a wizzy draft as html seeding the markdown document
					const draftMode = savedDraft.mode || appPrefs.defaultEditorMode || "wizzy";
					if (draftMode === "markdown") {
						mdSourceText = savedDraft.text;
						setEditorMode ("markdown");
						}
					else {
						setEditorMode ("wizzy");
						inputReplyComposer.html (savedDraft.text);
						mdSourceText = getMarkdownFromHtml (savedDraft.text);
						}
					}
				else {
					mdSourceText = "";
					setEditorMode (appPrefs.defaultEditorMode || "wizzy");
					inputReplyComposer.html ("");
					}
				savedMessage = undefined;
				flDraftChanged = false;
				updateModalStatus ();
				showModalWhen (new Date ());
				updateModalCharCount ();
				buttonReplyPost.text ("Publish");
				buttonReplyPost.prop ("disabled", getEditorText ().trim ().length === 0); //6/21/26 by CC -- #47: enable Publish when a restored draft already has text, not only after the next keystroke
				divReplyModal.addClass ("flNoParent");
				divReplyParent.hide ();
				divReplyParentDivider.hide ();
				replyTargetItem = undefined;
				lockBodyScroll ();
				divReplyOverlay.fadeIn (300);
				focusEditor ();
				if (currentEditorMode === "markdown") { //cursor at the end of the text, whichever surface
					const endPos = textareaMarkdown.val ().length;
					textareaMarkdown [0].setSelectionRange (endPos, endPos);
					}
				else {
					const range = document.createRange ();
					range.selectNodeContents (inputReplyComposer [0]);
					range.collapse (false);
					const selection = window.getSelection ();
					selection.removeAllRanges ();
					selection.addRange (range);
					}
				}
			function closeReplyModal (flDiscardDraft) {
				if (flDiscardDraft === true) { //6/25/26 by CC -- #87: publishing discards the draft; dismissing the editor (Cancel, Escape, click-away) keeps your text so the Show editor command can bring it back
					clearDraft ();
					}
				else {
					saveDraft (false); //6/25/26 by CC -- #87: keep the text but mark the editor dismissed, so a later reload won't auto-reopen it
					}
				divReplyOverlay.fadeOut (300, unlockBodyScroll); //unlock only after the overlay is gone, so the fading overlay can't become horizontally scrollable
				inputReplyComposer.html ("").removeClass ("flPublishing");
				textareaMarkdown.val ("").removeClass ("flPublishing"); //7/5/26 by CC -- #107
				mdSourceText = "";
				replyTargetItem = undefined;
				if (editTargetItem !== undefined) { //5/21/26 by Claude -- restore the new-post title snapshot
					currentTitle = savedTitleBeforeEdit;
					saveTitle ();
					refreshComposerTitle ();
					editTargetItem = undefined;
					savedTitleBeforeEdit = undefined;
					}
				if (theSpinner !== undefined) {
					theSpinner.stop ();
					theSpinner = undefined;
					}
				savedMessage = undefined;
				flDraftChanged = false;
				clearTimeout (draftSaveTimeout);
				divModalWhen.hide ();
				buttonReplyPost.prop ("disabled", true);
				}
			function openEditModal (item) { //5/21/26 by Claude
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				editTargetItem = item;
				savedTitleBeforeEdit = currentTitle;
				currentTitle = item.title;
				refreshComposerTitle ();

				const myScreenname = globals.myRssNetwork.getScreenname () || "?";
				const myInitial = myScreenname.charAt (0).toUpperCase ();
				populateAvatar (divReplyComposerAvatar, appPrefs.myAvatarImageUrl || appConsts.urlDefaultImage, myInitial);
				wireAvatarClick (divReplyComposerAvatar, myScreenname);

				setEditorMode ("wizzy");
				inputReplyComposer.html (item.description || "");
				inputReplyComposer.addClass ("flUserTouched");
				if ((typeof item.markdowntext === "string") && (item.markdowntext.length > 0)) { //7/5/26 by CC -- #107: the post's stored markdown is the document when we have it; otherwise derive it from the html once
					mdSourceText = item.markdowntext;
					}
				else {
					mdSourceText = getMarkdownFromHtml (item.description || "");
					}

				editOriginalTitle = item.title; //5/24/26 by Claude -- baseline for refreshUpdateDisabled
				editOriginalBody = inputReplyComposer.html (); //capture after the browser has normalized whatever we set
				editOriginalMd = mdSourceText; //7/5/26 by CC -- #107: change-detection baseline for markdown mode

				savedMessage = undefined;
				flDraftChanged = false;
				updateModalStatus ();
				showModalWhen (new Date (item.pubDate));
				updateModalCharCount ();
				buttonReplyPost.text ("Update");
				buttonReplyPost.prop ("disabled", true); //5/24/26 by Claude -- enabled only after user edits title or body
				divReplyModal.addClass ("flNoParent");
				divReplyParent.hide ();
				divReplyParentDivider.hide ();
				replyTargetItem = undefined;
				lockBodyScroll ();
				divReplyOverlay.fadeIn (300);
				inputReplyComposer.focus ();
				const range = document.createRange ();
				range.selectNodeContents (inputReplyComposer [0]);
				range.collapse (false);
				const selection = window.getSelection ();
				selection.removeAllRanges ();
				selection.addRange (range);
				}
			function updateMessageFromModal (htmlText, markdownText) { //5/21/26 by Claude
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				if (editTargetItem === undefined) {
					return;
					}
				const postRec = {
					id: editTargetItem.id,
					description: htmlText,
					markdowntext: markdownText
					};
				if (currentTitle !== undefined) {
					postRec.title = currentTitle;
					}
				savedMessage = "PUBLISHING";
				updateModalStatus ();
				theSpinner = new spinningImage ({});
				inputReplyComposer.addClass ("flPublishing");
				textareaMarkdown.addClass ("flPublishing"); //7/5/26 by CC -- #107
				buttonReplyPost.prop ("disabled", true);
				globals.myRssNetwork.updatePost (postRec, function (err, data) {
					if (theSpinner !== undefined) {
						theSpinner.stop ();
						theSpinner = undefined;
						}
					inputReplyComposer.removeClass ("flPublishing");
					textareaMarkdown.removeClass ("flPublishing"); //7/5/26 by CC -- #107
					savedMessage = undefined;
					flDraftChanged = false;
					updateModalStatus ();
					buttonReplyPost.prop ("disabled", false);
					if (err) {
						alertDialog (err.message);
						}
					else {
						speakerBeep (); //5/21/26 by Claude
						editTargetItem.title = postRec.title;
						if ((data !== undefined) && (data.description !== undefined)) { //7/13/26 by CC -- #175: the server linkifies bare URLs on save, so the repaint uses what it actually saved, not the editor's local copy
							editTargetItem.description = data.description;
							}
						else {
							editTargetItem.description = htmlText;
							}
						editTargetItem.markdowntext = markdownText;
						updateDisplayedItem (editTargetItem); //6/18/26 by Claude -- refresh the post in the timeline now, no socket round-trip
						showModalWhen (new Date ());
						closeReplyModal (true); //5/22/26 by Claude -- close dialog on save confirmation; 6/25 CC #87: publishing discards the draft
						}
					});
				}
			function postReply (parentId, htmlText, markdownText) {
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				savedMessage = "PUBLISHING";
				updateModalStatus ();
				theSpinner = new spinningImage ({});
				inputReplyComposer.addClass ("flPublishing");
				textareaMarkdown.addClass ("flPublishing"); //7/5/26 by CC -- #107
				buttonReplyPost.prop ("disabled", true);
				const postRec = {description: htmlText, markdowntext: markdownText, inReplyTo: parentId};
				if (currentTitle !== undefined) { //7/6/26 by CC -- #160: a reply carries its title too; this path silently dropped it, which is how post 191 published without one
					postRec.title = currentTitle;
					currentTitle = undefined;
					refreshComposerTitle ();
					saveTitle ();
					}
				globals.myRssNetwork.newPost (postRec, function (err, data) {
					if (err) {
						if (theSpinner !== undefined) {
							theSpinner.stop ();
							theSpinner = undefined;
							}
						inputReplyComposer.removeClass ("flPublishing");
						textareaMarkdown.removeClass ("flPublishing"); //7/5/26 by CC -- #107
						savedMessage = undefined;
						flDraftChanged = false;
						updateModalStatus ();
						buttonReplyPost.prop ("disabled", false);
						alertDialog (err.message);
						}
					else if (data !== undefined && data.guid !== undefined) {
						addMyPostToTimeline (data, undefined, htmlText, parentId); //6/18/26 by Claude -- show the reply now, no socket round-trip
						speakerBeep ();
						closeReplyModal (true); //6/25/26 by CC -- #87: publishing discards the draft
						goHomeWithNewPost (); //7/4/26 by CC -- #136: land the writer on the timeline where the reply just appeared
						}
					});
				}
			function goHomeWithNewPost () { //7/4/26 by CC -- #136: publishing always lands you on the home timeline with your new post at the top, whichever page you published from
				if (location.search.length > 0) { //on a story or profile url -- point the address bar home so reload and Back behave like the Home icon
					history.pushState ({}, "", location.origin);
					}
				if (!flTimelineLoaded) { //published from a cold-loaded story page -- the timeline holds only our quick local copy; clear it so the fresh load in viewHome renders every post once, in order
					divTimeline.empty ();
					Object.keys (itemsById).forEach (function (id) {
						delete itemsById [id];
						});
					Object.keys (itemsByGuid).forEach (function (theGuid) {
						delete itemsByGuid [theGuid];
						});
					}
				viewHome ();
				}

			function fillTitleDiv (divEl, theTitle, theLink, theGuid) {
				divEl.empty ();
				if (theTitle === undefined || theTitle.length === 0) {
					return;
					}
				if (theLink !== undefined && theLink.length > 0) { //6/29/26 by CC -- #118: an external link the writer added -- keep opening it in a new tab
					const aTitle = $('<a></a>').attr ("href", theLink).attr ("target", "_blank").attr ("rel", "noopener noreferrer").text (theTitle);
					divEl.append (aTitle);
					}
				else if (theGuid !== undefined && theGuid.length > 0 && (theGuid.indexOf ("http://") === 0 || theGuid.indexOf ("https://") === 0)) { //6/29/26 by CC -- #118: the post's own permalink -- open the story in place, like the eye icon, not a new tab
					const aTitle = $('<a></a>').attr ("href", theGuid).text (theTitle);
					aTitle.on ("click", function (theEvent) {
						theEvent.preventDefault ();
						openStoryInPlace (theGuid);
						});
					divEl.append (aTitle);
					}
				else {
					divEl.text (theTitle);
					}
				}
			function openStoryInPlace (theUrl) { //6/29/26 by CC -- #118: open a post as a story in place, the path the eye icon uses -- push the permalink so the address bar updates and Back returns to the timeline (popstate routes off the url), remember the scroll, then load the story
				history.pushState ({}, "", theUrl);
				savedTimelineScroll = window.scrollY;
				loadStory (theUrl);
				}
			function attachThreadClick (theThread) { //6/12/26 by Claude -- top-level click behavior, also reused when a reply is promoted to top level on delete
				theThread.on ("click", function (event) {
					const clickedTweetText = $(event.target).closest (".divTweetText");
					if (clickedTweetText.length > 0) {
						if (window.getSelection ().toString ().length === 0) { //7/14/26 by CC -- the click at the end of a drag-select must not toggle the post open
							const toggleFn = clickedTweetText.data ("toggleExpand");
							if (toggleFn !== undefined) {
								toggleFn ();
								}
							}
						}
					const divClickedThread = $(event.target).closest (".divThread"); //6/26/26 by CC -- the innermost item under the cursor, so clicking a reply selects the reply, not its parent thread
					const flAlreadySelected = (divSelectedThread !== undefined && divSelectedThread [0] === divClickedThread [0]);
					if (!flAlreadySelected) {
						selectThread (divClickedThread);
						}
					});
				}
			function paintLikeIcon (divIcon, flLiked) { //6/25/26 by CC -- #75: solid gray heart when the viewer has liked the post (so they can see it), open outline when not. The "liked" class also flags state for toggleLike.
				if (divIcon === undefined) {
					return;
					}
				const iTag = divIcon.find ("i");
				if (flLiked === true) {
					iTag.removeClass ("far").addClass ("fas");
					divIcon.addClass ("liked");
					}
				else {
					iTag.removeClass ("fas").addClass ("far");
					divIcon.removeClass ("liked");
					}
				}
			function eachRenderedHeart (idPost, visit) { //7/8/26 by CC -- the -1 like-count bug: find EVERY rendered copy of a post's heart, whatever view it's in, so no copy is left doing arithmetic on a stale count (#135 family)
				$(".divThread").each (function () {
					const divCopy = $(this);
					const copyItem = divCopy.data ("item");
					const likeUI = divCopy.data ("likeUI");
					if (copyItem !== undefined && copyItem.id === idPost && likeUI !== undefined && likeUI.divLikeIcon !== undefined) {
						visit (likeUI);
						}
					});
				}
			function setLikeCountDisplay (target, theCount) { //7/8/26 by CC -- one place writes the like count to the screen, and it never shows below zero whatever the arithmetic said
				if (theCount < 0) {
					theCount = 0;
					}
				target.likeCount = theCount;
				target.spanLikeCount.text (theCount);
				}
			function buildLikersText (screennames) { //6/25/26 by CC -- #75: comma-joined liker screennames, or the empty-state line
				if (screennames === undefined || screennames.length === 0) {
					return ("Not liked yet.");
					}
				var theText = "";
				screennames.forEach (function (theName) {
					if (theText.length > 0) {
						theText += ", ";
						}
					theText += theName;
					});
				return (theText);
				}
			function wireLikersTooltip (divIcon, theItem) { //6/25/26 by CC -- #75: on hover, fetch who liked this post and show their screennames in a tooltip (or "Not liked yet."); always current, nothing stored on the item
				var flHovering = false;
				divIcon.tooltip ({placement: "top", trigger: "manual", container: "body"});
				divIcon.on ("mouseenter", function () {
					flHovering = true;
					globals.myRssNetwork.getLikersList (theItem.id, function (err, screennames) {
						if (err !== undefined) {
							return;
							}
						if (flHovering === false) { //the pointer already left before the list came back
							return;
							}
						divIcon.attr ("data-original-title", buildLikersText (screennames));
						divIcon.tooltip ("show");
						});
					});
				divIcon.on ("mouseleave", function () {
					flHovering = false;
					divIcon.tooltip ("hide");
					});
				divIcon.on ("click", function () { //7/1/26 by CC -- #111: clicking the like icon hides its tooltip instead of leaving the stale "Not liked yet." stuck on screen
					flHovering = false;
					divIcon.tooltip ("hide");
					});
				}
			function popUpOrSelectParent (childItem) { //6/30/26 by CC -- #121: in the story view, climb one step up the thread. If the post this one replies to is already on screen (down the trail), just move the cursor up to it. Otherwise fetch that parent, insert it above as the new top, nest the current view under it (indented), and put the cursor on it -- so you walk toward the root one step at a time, leaving the trail behind you. Home resets.
				const parentId = childItem.inReplyTo;
				if (parentId === undefined) {
					return; //nothing above -- this post is the root of the thread
					}
				var divExistingParent; //the parent's thread, if it's already rendered somewhere in the story
				divStory.find (".divThread").toArray ().forEach (function (theElement) {
					const theThreadItem = $(theElement).data ("item");
					if ((theThreadItem !== undefined) && (theThreadItem.id === parentId)) {
						divExistingParent = $(theElement);
						}
					});
				if (divExistingParent !== undefined) {
					selectThread (divExistingParent, true); //already on the trail -- just move the cursor up to it
					}
				else {
					const currentTopThreads = divStory.children (".divThread"); //the trail so far -- becomes the parent's nested replies
					globals.myRssNetwork.getItemByGuid (location.origin + "?id=" + parentId, function (err, parentItem) {
						if (err) {
							console.log ("popUpOrSelectParent: err.message == " + err.message);
							history.pushState ({}, "", location.origin + "?id=" + parentId); //7/7/26 by CC -- the climb hit a post the server won't serve (deleted); land on its story page so the reader sees the message instead of a silent dead click; Back returns
							loadStory (location.origin + "?id=" + parentId);
							}
						else {
							if ((parentItem === undefined) || (parentItem.id === undefined)) {
								console.log ("popUpOrSelectParent: no post with id == " + parentId);
								}
							else {
								addItemToTimeline (parentItem, divStory); //render the parent -- lands at the bottom of the story area for now
								const divParentThread = divStory.children (".divThread").last ();
								divStory.prepend (divParentThread); //move it to the top
								divParentThread.children (".divReplies").append (currentTopThreads); //nest the old view under it, indented
								selectThread (divParentThread, true); //cursor on the newly-added post -- the next step toward the summit
								}
							}
						});
					}
				}
			var itemSeenObserver; //assigned on first use by watchItemSeen
			function watchItemSeen (divThread, item) { //7/2/26 by CC -- #129: read means it was on your screen; tell the app when a post has actually been visible. Half the post on screen counts; a post taller than the window counts when it fills half the window
				if (options.itemSeenCallback === undefined) {
					return;
					}
				if (itemSeenObserver === undefined) {
					itemSeenObserver = new IntersectionObserver (function (theEntries) {
						theEntries.forEach (function (theEntry) {
							var flSeen = false;
							if (theEntry.intersectionRatio >= 0.5) {
								flSeen = true;
								}
							else {
								if (theEntry.isIntersecting && (theEntry.intersectionRect.height >= (window.innerHeight / 2))) {
									flSeen = true;
									}
								}
							if (flSeen) {
								const seenItem = $(theEntry.target).data ("item");
								if (seenItem !== undefined) {
									options.itemSeenCallback (seenItem);
									}
								itemSeenObserver.unobserve (theEntry.target);
								}
							});
						}, {threshold: [0, 0.5]});
					}
				itemSeenObserver.observe (divThread [0]);
				}
			function selectNewestItem () { //7/3/26 by CC -- #133: arriving at the timeline, the cursor sits on the newest post at the top; the read bookmark keeps recording invisibly via watchItemSeen
				var newestEntry;
				Object.keys (itemsById).forEach (function (id) { //ascending id order is chronological; the last one is the newest
					newestEntry = itemsById [id];
					});
				if (newestEntry !== undefined) {
					selectThread (newestEntry.divThread, true);
					}
				}

			function addItemToTimeline (item, targetContainer) { //6/20/26 by Claude -- targetContainer set means render this one post into its own area (the story page) instead of the timeline
				if (item.inReplyTo === undefined && item.inReplyToNum !== undefined) { //6/25/26 by CC -- the server names the parent's id inReplyToNum; thread on it so replies nest on reload, not just on the live socket path
					item.inReplyTo = item.inReplyToNum;
					}
				const theDate = new Date (item.pubDate);
				const author = item.author || "?";
				const initial = author.charAt (0).toUpperCase ();
				const parentEntry = (item.inReplyTo === undefined) ? undefined : itemsById [item.inReplyTo];

				const divThread = $('<div class="divThread"></div>');
				divThread.data ("item", item); //6/26/26 by CC -- the cursor points at one item; every thread (top-level, reply, story, profile) carries its own item so it can be selected and edited individually
				const divTweet = $('<div class="divTweet"></div>');
				const divAvatar = $('<div class="divAvatar"></div>');
				populateAvatar (divAvatar, item.imageUrl || appConsts.urlDefaultImage, initial);
				wireAvatarClick (divAvatar, item.screenname);
				const divTweetBody = $('<div class="divTweetBody"></div>');

				var parentAuthor; //display name of who this post replies to -- 6/30/26 by CC #121
				if (parentEntry === undefined) { //parent isn't on screen (the story view, or a timeline reply whose parent isn't loaded) -- use the name the server computed
					parentAuthor = item.inReplyToAuthor;
					}
				else { //parent is loaded -- use its display name, matching the existing timeline behavior
					parentAuthor = parentEntry.item.author || "?";
					}
				if (parentAuthor !== undefined) {
					const divReplyingTo = $('<div class="divReplyingTo"></div>').text ("Replying to @" + parentAuthor);
					if (item.inReplyTo !== undefined) { //6/30/26 by CC #121: click the line to reach the post this one replies to -- the message above it
						divReplyingTo.on ("click", function (theEvent) {
							theEvent.preventDefault ();
							theEvent.stopPropagation ();
							if (divChat.hasClass ("storyPage") === true) { //story view: climb one step up the thread, leaving the trail below
								popUpOrSelectParent (item);
								}
							else { //timeline: open the replied-to post as a story in place
								openStoryInPlace (location.origin + "?id=" + item.inReplyTo);
								}
							});
						}
					divTweetBody.append (divReplyingTo);
					}

				const divTweetHeader = $('<div class="divTweetHeader"></div>');
				const spanAuthor = $('<span class="spanAuthor"></span>');
				spanAuthor.text (author);
				if (item.screenname !== undefined) { //7/16/26 by CC -- the name goes to the person's timeline, same as the avatar; their website is linked from the profile page
					wireAvatarClick (spanAuthor, item.screenname);
					}
				if (item.screenname !== undefined) { //7/1/26 by CC -- #113: the @handle no longer sits on the top line; it moves into the name's hover, followed by the feed description when there is one
					var toolTipText = "@" + item.screenname;
					if ((item.feedDescription !== undefined) && (item.feedDescription.length > 0)) {
						toolTipText += " — " + item.feedDescription; //em dash between the handle and the description
						}
					addToolTip (spanAuthor, toolTipText, "bottom");
					}
				const spanTimestamp = $('<span class="spanTimestamp"></span>');
				spanTimestamp.text (String.fromCharCode (0x00B7) + " "); //middle dot
				const aTimestamp = $('<a class="aTimestamp"></a>').attr ("href", location.origin + "?id=" + item.id); //6/29/26 by CC -- #118: link the time to the permalink, opened in place like the eye icon instead of a new tab
				aTimestamp.append (getUpdateableTime (theDate, "", true)); //no trailing period; the live-updating time sits inside the link
				aTimestamp.on ("click", function (theEvent) {
					theEvent.preventDefault ();
					openStoryInPlace (location.origin + "?id=" + item.id);
					});
				spanTimestamp.append (aTimestamp);
				divTweetHeader.append (spanAuthor);
				divTweetHeader.append (spanTimestamp);
				if (item.feedUrl !== undefined) { //7/16/26 by CC -- a feed icon after the time, linked to the author's feed
					const spanFeedIcon = $('<span class="spanFeedIcon"></span>');
					spanFeedIcon.text (String.fromCharCode (0x00B7)); //middle dot -- the space after it comes from the icon's margin, so it matches the flex gap before it
					const aFeedIcon = $('<a class="aFeedIcon" target="_blank"><i class="fas fa-rss"></i></a>').attr ("href", item.feedUrl);
					addToolTip (aFeedIcon, author + "'s " + appConsts.productNameForDisplay + " feed", "bottom");
					spanFeedIcon.append (aFeedIcon);
					divTweetHeader.append (spanFeedIcon);
					}

				const divItemMenu = $('<div class="divContextMenu divItemMenu"></div>');
				const buttonItemMenu = $('<button class="buttonItemMenu" title="More"><i class="fas fa-ellipsis-v"></i></button>');
				const divItemMenuPopup = $('<div class="divContextMenuPopup"></div>');
				if (options.timelineItemMenu !== undefined) { //6/10/26 by Claude -- menu entries come from the structure the app passes in
					fillMenuPopup (divItemMenuPopup, options.timelineItemMenu, function (ev) {
						ev.item = item;
						});
					}
				divItemMenu.append (buttonItemMenu);
				divItemMenu.append (divItemMenuPopup);

				buttonItemMenu.on ("mousedown", function (event) {
					event.preventDefault ();
					});
				buttonItemMenu.on ("click", function (event) {
					event.stopPropagation ();
					selectThread (divThread); //clicking the menu also moves the cursor to this item
					const flWasOpen = divItemMenuPopup.hasClass ("flOpen");
					$(".divContextMenuPopup").removeClass ("flOpen");
					if (!flWasOpen) {
						refreshIconsEnabled (); //6/10/26 by Claude -- mirror enabled state right now, so the menu opens correct instead of waiting for the next tick
						divItemMenuPopup.addClass ("flOpen");
						}
					});

				const divTweetTitle = $('<div class="divTweetTitle"></div>');
				fillTitleDiv (divTweetTitle, item.title, item.link, item.guid);
				const divTweetText = $('<div class="divTweetText"></div>').html (item.description);
				divTweetText.on ("mousedown", function (event) {
					if (event.detail > 1) { //suppress word-select on rapid double-click
						event.preventDefault ();
						}
					});

				const divMoreButton = $('<div class="divMoreButton"></div>');
				var flExpanded = false;
				var flCanExpand = false;
				function setMoreLabel () {
					if (flExpanded === true) {
						divMoreButton.html ("LESS ↑"); //upwards arrow
						}
					else {
						divMoreButton.html ("MORE ↓"); //downwards arrow
						}
					}
				function toggleExpand () {
					if (flExpanded === false) { //7/7/26 by CC -- re-measure before deciding: the render-time measure runs before images load (and can run while the view is hidden), which left image-bearing posts clipped with a dead click (id=191 was the repro)
						setupMoreButton ();
						}
					if (flCanExpand === false) {
						return;
						}
					if (flExpanded === true) {
						divTweetText.css ("max-height", "");
						divTweetText.addClass ("bodyTruncated");
						flExpanded = false;
						}
					else {
						divTweetText.css ("max-height", divTweetText.prop ("scrollHeight") + "px");
						divTweetText.removeClass ("bodyTruncated");
						flExpanded = true;
						}
					setMoreLabel ();
					}
				function expandFully () { //called when an edit updates the item -- show full content
					flCanExpand = true;
					if (false === true) {
						divMoreButton.css ("display", "block");
						}
					divTweetText.css ("max-height", divTweetText.prop ("scrollHeight") + "px");
					divTweetText.removeClass ("bodyTruncated");
					flExpanded = true;
					setMoreLabel ();
					}
				function refreshAfterUpdate () { //7/5/26 by CC -- #153: the content changed in place (a like or edit came back from the server); keep the reader's expanded/collapsed state instead of forcing the post open, so the icons don't jump down the screen
					if (flExpanded === true) {
						divTweetText.css ("max-height", divTweetText.prop ("scrollHeight") + "px"); //re-measure; the new content may be a different height
						}
					else {
						divTweetText.css ("max-height", "");
						setupMoreButton ();
						}
					}
				function setupMoreButton () {
					const scrollHeight = divTweetText.prop ("scrollHeight");
					const clientHeight = divTweetText.prop ("clientHeight");
					if (scrollHeight > clientHeight) {
						flCanExpand = true;
						divTweetText.addClass ("bodyTruncated");
						if (false === true) {
							divMoreButton.css ("display", "block");
							setMoreLabel ();
							}
						}
					else {
						flCanExpand = false;
						divMoreButton.css ("display", "none");
						divTweetText.removeClass ("bodyTruncated");
						}
					}
				divMoreButton.on ("click", function (ev) {
					ev.stopPropagation ();
					toggleExpand ();
					});
				divTweetText.data ("toggleExpand", toggleExpand);

				const divTweetActions = $('<div class="divTweetActions"></div>');
				const spanReplyCount = $('<span class="spanActionCount"></span>').text (item.ctReplies || 0); //5/22/26 by Claude -- count badge follows the comment icon. 7/3/26 by CC -- starts from the server's count, so it's right in every view no matter what else is loaded
				const spanLikeCount = $('<span class="spanActionCount">0</span>'); //6/25/26 by CC -- #75: like count badge follows the heart, like the comment count
				var divLikeIcon; //6/25/26 by CC -- #75: the like heart for this post, captured below so toggleLike can repaint it

				const divWedgeReplies = $('<div class="divWedgeReplies"><i class="fas fa-caret-right"></i></div>'); //7/9/26 by CC -- the wedge left of the comment icon: dark means the post has replies you can't see, light means nothing hidden (the Frontier/Drummer "don't bother clicking here" signal); click a dark wedge to open the replies in place, click again to put them away
				var flFetchingReplies = false; //true while an expand fetch is in flight, so a double-click can't fetch twice
				var hiddenReplyRows = new Array (); //7/10/26 by CC -- timeline rows folded away while this wedge is open, restored on close
				function restoreHiddenRows () { //7/10/26 by CC -- put back the timeline rows this wedge folded away, and any folded by wedges nested inside it (collapsing a parent must not strand a deeper wedge's rows)
					divReplies.find (".divThread").each (function () {
						const nestedWedgeUI = $(this).data ("wedgeUI");
						if (nestedWedgeUI !== undefined) {
							nestedWedgeUI.restoreHiddenRows ();
							}
						});
					hiddenReplyRows.forEach (function (divRow) {
						divRow.show ();
						});
					hiddenReplyRows = new Array ();
					}
				function paintWedge () {
					const ctReplies = Number (spanReplyCount.text ()) || 0;
					const ctVisible = divReplies.children (".divThread").length;
					if (ctVisible > 0) { //replies are open underneath -- light, click puts them away
						divWedgeReplies.removeClass ("wedgeDark");
						divWedgeReplies.addClass ("wedgeClickable");
						}
					else {
						if (ctReplies > 0) { //replies exist but you can't see them -- dark, click opens them
							divWedgeReplies.addClass ("wedgeDark wedgeClickable");
							}
						else { //nothing underneath
							divWedgeReplies.removeClass ("wedgeDark wedgeClickable");
							}
						}
					scheduleRenderMap (); //7/9/26 by CC -- every render, expansion, collapse and count change lands here; the map follows along
					}
				function closeReplies () { //7/10/26 by CC -- put the nest away if it's open, nothing if it's not -- split from toggleReplies so Home can close every open wedge without toggling closed ones open
					if (divReplies.children (".divThread").length > 0) {
						restoreHiddenRows ();
						const flCursorInside = (divSelectedThread !== undefined) && ($.contains (divReplies [0], divSelectedThread [0])); //the cursor mustn't vanish with the nest (the map's wedge can close without moving it first)
						divReplies.empty ();
						if (flCursorInside) {
							selectThread (divThread);
							}
						paintWedge ();
						}
					}
				function toggleReplies () {
					if (divReplies.children (".divThread").length > 0) { //open -- put them away
						closeReplies ();
						}
					else {
						const ctReplies = Number (spanReplyCount.text ()) || 0;
						if ((ctReplies > 0) && !flFetchingReplies) { //fetch one level of replies and open them in place; each reply carries its own wedge, so the walk goes as deep as you want
							flFetchingReplies = true;
							globals.myRssNetwork.getItemAndReplies (globals.myRssNetwork.getScreenname (), item.id, function (err, theItems) {
								flFetchingReplies = false;
								if (err) {
									console.log ("toggleReplies: err.message == " + err.message);
									}
								else {
									if (theItems !== undefined) {
										var replyItems = new Array ();
										theItems.forEach (function (oneItem) {
											if (oneItem.id !== item.id) { //the call returns the post itself along with its replies
												replyItems.push (oneItem);
												}
											});
										replyItems.reverse (); //newest reply at the top, matching the story view and the timeline
										const flInTimeline = $.contains (divTimeline [0], divThread [0]); //7/10/26 by CC -- only the timeline shows a reply twice (its own row + the nest); other views have no rows to fold
										replyItems.forEach (function (replyItem) {
											addItemToTimeline (replyItem, divReplies);
											if (flInTimeline) { //7/10/26 by CC -- the reply may already be on screen as its own timeline row; fold it into the nest so every post shows exactly once, cursor following if it was there
												const entry = itemsById [replyItem.id];
												if ((entry !== undefined) && (entry.divThread.css ("display") !== "none")) {
													if ((divSelectedThread !== undefined) && (divSelectedThread [0] === entry.divThread [0])) {
														const divNestedCopy = divReplies.children (".divThread").last ();
														selectThread (divNestedCopy);
														}
													entry.divThread.hide ();
													hiddenReplyRows.push (entry.divThread);
													}
												}
											});
										}
									paintWedge ();
									}
								});
							}
						}
					}
				divWedgeReplies.on ("click", function (ev) {
					ev.stopPropagation ();
					selectThread (divThread); //clicking the wedge also moves the cursor to this item, like the other icons
					toggleReplies ();
					});
				divThread.data ("wedgeUI", {paintWedge, toggleReplies, closeReplies, restoreHiddenRows, divWedgeReplies}); //so the story and structure renderers can repaint after nesting replies under this post, and the map can twin the wedge. 7/10/26 by CC -- restoreHiddenRows added so a collapsing ancestor can put back rows a nested wedge folded away; closeReplies so Home can put every open wedge away

				if (options.itemIconBar !== undefined) {
					options.itemIconBar.forEach (function (iconDef) {
						const divInsideItemIcon = $('<div class="divInsideItemIcon"></div>').html (iconDef.icon);
						divInsideItemIcon.data ("iconDef", iconDef); //6/1/26 by Claude -- per-second refresh reads enabled from here
						if (iconDef.tooltip !== undefined) {
							divInsideItemIcon.attr ("title", iconDef.tooltip);
							}
						divInsideItemIcon.on ("click", function (ev) {
							if (iconDef.enabled !== true) { //disabled icons ignore clicks
								return;
								}
							selectThread (divThread); //clicking an icon also moves the cursor to this item
							ev.item = item;
							ev.iconName = iconDef.name;
							iconDef.click (ev);
							});
						if (iconDef.name === "like") { //6/25/26 by CC -- #75: capture the heart, paint its initial liked/not state, show the like count, and wire the who-liked hover tooltip
							divLikeIcon = divInsideItemIcon;
							divInsideItemIcon.append (spanLikeCount);
							paintLikeIcon (divLikeIcon, item.flLiked);
							spanLikeCount.text (item.ctLikes || 0);
							wireLikersTooltip (divLikeIcon, item);
							}
						if (iconDef.name === "comment") { //7/9/26 by CC -- the comment icon travels with its wedge as one unit, so the icon row's spacing treats the pair as a single stop
							divInsideItemIcon.append (spanReplyCount);
							const divCommentGroup = $('<div class="divCommentGroup"></div>');
							divCommentGroup.append (divWedgeReplies);
							divCommentGroup.append (divInsideItemIcon);
							divTweetActions.append (divCommentGroup);
							}
						else {
							divTweetActions.append (divInsideItemIcon);
							}
						});
					}
				divTweetActions.append (divItemMenu);
				divThread.data ("likeUI", {divLikeIcon, spanLikeCount, likeCount: (item.ctLikes || 0)}); //7/4/26 by CC -- #135: every rendered copy carries its own heart, so toggleLike can repaint it in any view, not just the timeline

				const divReplies = $('<div class="divReplies"></div>');

				divTweetBody.append (divTweetHeader);
				if (item.title !== undefined && item.title.length > 0) {
					divTweetBody.append (divTweetTitle);
					}
				divTweetBody.append (divTweetText);
				divTweetBody.append (divMoreButton);
				divTweetBody.append (divTweetActions);
				divTweet.append (divAvatar);
				divTweet.append (divTweetBody);
				divThread.append (divTweet);
				divThread.append (divReplies);
				paintWedge (); //7/9/26 by CC -- first paint, now that the replies container exists

				if (targetContainer === undefined) { //timeline -- 7/2/26 by CC: #129 flat reverse-chron; every post renders at the top level, replies included, newest at top. The Replying-to line carries the context; the story view holds the structure
					divTimeline.prepend (divThread);
					attachThreadClick (divThread);
					watchItemSeen (divThread, item); //#129: read means it was visible on screen -- keeps the read bookmark current for later use, nothing visible (#133)
					divThread.data ("itemId", item.id); //6/12/26 by Claude -- lets removeItem map a promoted reply's element back to its entry
					itemsById [item.id] = {divThread, divReplies, spanReplyCount, replyCount: (item.ctReplies || 0), item, setupMoreButton, expandFully, refreshAfterUpdate, divLikeIcon, spanLikeCount, likeCount: (item.ctLikes || 0), paintWedge};
					if (item.guid !== undefined) {
						itemsByGuid [item.guid] = itemsById [item.id];
						}
					}
				else { //6/20/26 by Claude -- story page: render into its own area and don't register in the timeline's item maps, so the timeline underneath stays untouched
					targetContainer.append (divThread);
					attachThreadClick (divThread);
					}
				setupMoreButton ();
				}

			function bumpParentReplyCount (theItem) { //7/3/26 by CC -- a reply just arrived live; the parent's server-carried count predates it, so nudge the badge by hand. Loads don't call this -- their counts come complete from the server
				if (theItem.inReplyTo !== undefined) {
					const parentEntry = itemsById [theItem.inReplyTo];
					if (parentEntry !== undefined) {
						parentEntry.replyCount = parentEntry.replyCount + 1;
						parentEntry.spanReplyCount.text (parentEntry.replyCount);
						if (parentEntry.paintWedge !== undefined) { //7/9/26 by CC -- the parent just gained a reply; its wedge may go dark
							parentEntry.paintWedge ();
							}
						}
					}
				}

			function addMyPostToTimeline (data, theTitle, htmlText, inReplyTo) { //6/18/26 by Claude -- show the user's own just-published post immediately, built from local identity; the socket copy that follows is deduped by guid in newItem
				if (data.guid !== undefined && itemsByGuid [data.guid] !== undefined) { //6/21/26 by CC -- the inline socket can show this post before the publish response returns (the response waits on the S3 feed write); don't add a second copy
					return;
					}
				const myScreenname = globals.myRssNetwork.getScreenname ();
				const chatItem = {
					id: data.id,
					guid: data.guid,
					feedUrl: globals.myRssNetwork.getFeedUrl (),
					author: appPrefs.myFeedTitle || myScreenname,
					screenname: myScreenname,
					title: theTitle,
					description: htmlText,
					pubDate: data.pubDate || new Date ().toISOString (),
					imageUrl: appPrefs.myAvatarImageUrl,
					feedLink: appPrefs.myFeedLink,
					feedDescription: appPrefs.myFeedDescription,
					inReplyTo: inReplyTo
					};
				addItemToTimeline (chatItem);
				bumpParentReplyCount (chatItem); //7/3/26 by CC -- our own reply is newer than the parent's server count
				}

			function sendMessage () {
				const html = inputComposer.html ();
				const text = inputComposer.text ().trim ();
				if (text.length === 0) {
					return;
					}
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				inputComposer.html ("");
				clearDraft ();
				const postRec = {description: html};
				if (currentTitle !== undefined) {
					postRec.title = currentTitle;
					currentTitle = undefined;
					refreshComposerTitle ();
					saveTitle ();
					}
				globals.myRssNetwork.newPost (postRec, function (err, data) {
					if (err) {
						console.log ("sendMessage: err.message == " + err.message);
						}
					else if (data !== undefined && data.guid !== undefined) {
						pendingReplyMap [data.guid] = {inReplyTo: undefined, id: data.id};
						}
					});
				}

			function sendMessageFromModal (htmlText, markdownText) {
				if (!globals.myRssNetwork.userIsSignedIn ()) {
					return;
					}
				clearDraft ();
				const postRec = {description: htmlText, markdowntext: markdownText};
				if (currentTitle !== undefined) {
					postRec.title = currentTitle;
					currentTitle = undefined;
					refreshComposerTitle ();
					saveTitle ();
					}
				savedMessage = "PUBLISHING";
				updateModalStatus ();
				theSpinner = new spinningImage ({});
				inputReplyComposer.addClass ("flPublishing");
				textareaMarkdown.addClass ("flPublishing"); //7/5/26 by CC -- #107
				buttonReplyPost.prop ("disabled", true);
				globals.myRssNetwork.newPost (postRec, function (err, data) {
					if (err) {
						if (theSpinner !== undefined) {
							theSpinner.stop ();
							theSpinner = undefined;
							}
						inputReplyComposer.removeClass ("flPublishing");
						textareaMarkdown.removeClass ("flPublishing"); //7/5/26 by CC -- #107
						savedMessage = undefined;
						flDraftChanged = false;
						updateModalStatus ();
						buttonReplyPost.prop ("disabled", false);
						alertDialog (err.message);
						}
					else if (data !== undefined && data.guid !== undefined) {
						addMyPostToTimeline (data, postRec.title, htmlText, undefined); //6/18/26 by Claude -- show the post now, no socket round-trip
						speakerBeep ();
						closeReplyModal (true); //6/25/26 by CC -- #87: publishing discards the draft
						goHomeWithNewPost (); //7/4/26 by CC -- #136: land the writer on the timeline where the post just appeared
						}
					});
				}

			function updateForLogin () {
				const flSignedIn = globals.myRssNetwork.userIsSignedIn ();
				divUserProfile.empty ();
				divFeedCard.empty ();
				divFeedCard.append ($('<div class="divCardTitle">Subscribe via RSS</div>'));
				if (flSignedIn) {
					inputComposer.attr ("contenteditable", "false");
					inputComposer.attr ("data-placeholder", options.placeholderSignedIn);
					inputComposer.css ("cursor", "pointer");
					inputComposer.removeClass ("flDisabled");
					buttonPost.prop ("disabled", false);

					const screenname = globals.myRssNetwork.getScreenname () || "?";
					const initial = screenname.charAt (0).toUpperCase ();
					populateAvatar (divComposerAvatar, appConsts.urlDefaultImage, initial);
					wireAvatarClick (divComposerAvatar, screenname);

					const divProfileAvatar = $('<div class="divProfileAvatar"></div>');
					populateAvatar (divProfileAvatar, appConsts.urlDefaultImage, initial);
					wireAvatarClick (divProfileAvatar, screenname);
					const divProfileInfo = $('<div class="divProfileInfo"></div>');
					divProfileInfo.append ($('<div class="divProfileName"></div>').text (screenname));
					divProfileInfo.append ($('<div class="divProfileHandle"></div>').text ("@" + screenname));
					const buttonSignOut = $('<button class="buttonSignOut">Sign out</button>');
					buttonSignOut.on ("click", function () {
						globals.myRssNetwork.signOut ();
						});
					divUserProfile.append (divProfileAvatar);
					divUserProfile.append (divProfileInfo);
					divUserProfile.append (buttonSignOut);

					const feedUrl = globals.myRssNetwork.getFeedUrl ();
					divFeedCard.append ($('<a class="aFeedLink" target="_blank"></a>').attr ("href", feedUrl).text (feedUrl));
					}
				else {
					inputComposer.attr ("contenteditable", "false");
					inputComposer.attr ("data-placeholder", options.placeholderSignedOut);
					inputComposer.addClass ("flDisabled");
					buttonPost.prop ("disabled", true);
					populateAvatar (divComposerAvatar, appConsts.urlDefaultImage, "?");
					divUserProfile.append ($('<div class="divProfileSignedOut">Not signed in</div>'));
					divFeedCard.append ($('<div class="divCardBody">Sign in to get your feed URL</div>'));
					}
				}

			function loadRecentItems (callback) { //6/27/26 by CC -- optional callback fires once the timeline is loaded, so the scanner can open over a populated itemsById
				globals.myRssNetwork.getRecentItems (options.ctRecentItems, function (err, theItems) {
					if (err) {
						console.log ("loadRecentItems: err.message == " + err.message);
						restorePendingContextDraft (); //6/21/26 by CC -- items failed to load; still let a waiting draft restore (text preserved even if its target post is missing)
						}
					else {
						theItems.reverse (); //oldest first so a reply's parent is added before the reply
						theItems.forEach (function (item) {
							addItemToTimeline (item);
							});
						flTimelineLoaded = true; //6/29/26 by CC -- #118: timeline is populated; Home and Back can reveal it
						selectNewestItem (); //7/3/26 by CC -- #133: the cursor always starts on the top post
						restorePendingContextDraft (); //6/21/26 by CC -- items are in; reopen a reply/edit draft against its target post
						}
					if (callback !== undefined) {
						callback ();
						}
					});
				}

			function renderRepliesList (parentItem, theContainer) { //6/27/26 by CC -- #102: under a focused post, list its direct replies (one level), each tagged with a parenthetical count of its own direct replies. In-app the replies are already loaded in itemsById; a cold-loaded view (no timeline behind it) shows none until we fetch them from the server
				function countDirectReplies (parentId) {
					var ct = 0;
					Object.keys (itemsById).forEach (function (id) {
						if (itemsById [id].item.inReplyTo === parentId) {
							ct++;
							}
						});
					return (ct);
					}
				var theReplies = new Array ();
				Object.keys (itemsById).forEach (function (id) { //ascending id order is chronological -- oldest reply first
					const entry = itemsById [id];
					if (entry.item.inReplyTo === parentItem.id) {
						theReplies.push (entry.item);
						}
					});
				theReplies.reverse (); //6/27/26 by CC -- newest reply at the top, to match the timeline
				theReplies.forEach (function (replyItem) {
					addItemToTimeline (replyItem, theContainer);
					const ctChildren = countDirectReplies (replyItem.id);
					if (ctChildren > 0) {
						const divLastThread = theContainer.children (".divThread").last ();
						const spanHint = $('<span class="spanReplyCountHint"></span>').text (" (" + ctChildren + ")");
						divLastThread.find (".divTweetHeader").first ().append (spanHint);
						}
					});
				}
			function getDirectReplies (parentId) { //6/27/26 by CC -- the posts that reply directly to parentId, newest first so the newest reply shows at the top -- matches the timeline
				var theReplies = new Array ();
				Object.keys (itemsById).forEach (function (id) { //ascending id order is chronological
					const entry = itemsById [id];
					if (entry.item.inReplyTo === parentId) {
						theReplies.push (entry.item);
						}
					});
				theReplies.reverse (); //newest first
				return (theReplies);
				}
			function renderSubtree (theItem, theContainer) { //6/27/26 by CC -- #104: render a post and its entire nested reply structure into theContainer, so the focused view shows the whole conversation, not just the direct replies
				addItemToTimeline (theItem, theContainer);
				const divItemThread = theContainer.children (".divThread").last ();
				const divItemReplies = divItemThread.children (".divReplies");
				getDirectReplies (theItem.id).forEach (function (reply) {
					renderSubtree (reply, divItemReplies);
					});
				const wedgeUI = divItemThread.data ("wedgeUI"); //7/9/26 by CC -- replies just nested underneath; repaint the wedge
				if (wedgeUI !== undefined) {
					wedgeUI.paintWedge ();
					}
				}
			function openScannerArea () { //6/27/26 by CC -- reveal the scanner over the timeline, mirroring the story area
				divTimeline.hide ();
				divStory.hide ();
				divProfile.hide ();
				divScanner.show ();
				divChat.addClass ("scannerPage");
				}
			function returnToTimelineAtItem (itemId) { //6/27/26 by CC -- #104: leave the scanner (hoisted view) and show the timeline, cursor on the given post -- dehoist's way back out when the stack empties
				divChat.removeClass ("scannerPage");
				divScanner.hide ();
				divScanner.empty ();
				divTimeline.show ();
				scheduleRenderMap (); //7/9/26 by CC -- back on the timeline; nothing re-renders, so nudge the map by hand
				const entry = itemsById [itemId];
				if (entry !== undefined) {
					selectThread (entry.divThread, true);
					}
				else {
					window.scrollTo (0, savedTimelineScroll);
					}
				}
			function renderHoistedView (itemId) { //6/27/26 by CC -- #104 hoist: show the post as the temporary root -- it and its whole reply tree, nothing else -- rendered timeline-style into the scanner area. No crumb trail; the Dehoist icon's enabled state is the only indicator
				const entry = itemsById [itemId];
				if (entry === undefined) {
					return;
					}
				divScanner.empty ();
				renderSubtree (entry.item, divScanner);
				openScannerArea ();
				const divRootThread = divScanner.children (".divThread").first (); //the hoisted root; its nested replies live inside it
				selectThread (divRootThread, true); //cursor on the root
				}
			function logHoistStack (theVerb) { //6/27/26 by CC -- #104 debug: after a hoist/dehoist, dump the stack (outermost first) with each level's title, or its description text when there's no title
				console.log (theVerb + " -- hoist stack (" + hoistStack.length + "):");
				hoistStack.forEach (function (id, level) {
					const entry = itemsById [id];
					var theText = "(not loaded)";
					if (entry !== undefined) {
						const theItem = entry.item;
						if ((theItem.title !== undefined) && (theItem.title.length > 0)) {
							theText = theItem.title;
							}
						else {
							theText = $('<div></div>').html (theItem.description || "").text ().trim ();
							}
						}
					console.log ("  [" + level + "] id " + id + ": " + theText);
					});
				}
			function loadStory (guid) { //6/19/26 by Claude -- standalone story page: fetch one post and render it full-length, nothing hidden; 6/20 renders into its own area and hides the timeline, leaving it untouched underneath. 6/30/26 by CC -- #121: a reply opened as its parent's story with the reply nested inside. 7/10/26 by CC -- undone: a post's link shows THAT post, reply or not, with its own replies below; the Replying-to link climbs up
				function openStoryArea () { //6/21/26 by CC -- reveal the story area over the timeline
					divTimeline.hide ();
					divStory.show ();
					divChat.addClass ("storyPage");
					window.scrollTo (0, 0);
					}
				function showStoryNotFound (theMessage) { //6/21/26 by CC -- the guid didn't resolve to a post; show a clear message instead of an empty item. 7/7/26 by CC -- the server can say why (a deleted post gets its own message); generic text when it doesn't
					if (theMessage === undefined) {
						theMessage = "Can't view the story because it doesn't appear to exist. :-)";
						}
					divStory.empty ();
					divStory.append ($('<div class="divStoryNotFound"></div>').text (theMessage));
					openStoryArea ();
					}
				function renderSingleStory (theItem) { //6/30/26 by CC -- #121 fallback: the post stands on its own (the context fetch came back without the root, e.g. a deleted parent) -- show just it, the pre-#121 behavior
					divStory.empty ();
					addItemToTimeline (theItem, divStory);
					renderRepliesList (theItem, divStory); //6/27/26 by CC -- #102: list the post's direct replies under it
					openStoryArea ();
					}
				function renderStoryWithContext (theItems, idRoot, idLinked) { //6/30/26 by CC -- #121: render the root post and its direct replies into the story area, then drop the cursor on the post the link pointed at (the root itself, or one of the replies)
					divStory.empty ();
					var rootItem; //the post everything hangs off -- the parent when the link pointed at a reply, otherwise the linked post itself
					var replyItems = new Array ();
					theItems.forEach (function (oneItem) {
						if (oneItem.id === idRoot) {
							rootItem = oneItem;
							}
						else {
							replyItems.push (oneItem);
							}
						});
					addItemToTimeline (rootItem, divStory);
					const divRootThread = divStory.children (".divThread").last ();
					const divRootReplies = divRootThread.children (".divReplies");
					var divLinkedThread; //the thread to put the cursor on -- assigned when we render the linked post
					if (rootItem.id === idLinked) {
						divLinkedThread = divRootThread;
						}
					replyItems.reverse (); //newest reply at the top, to match the timeline
					replyItems.forEach (function (replyItem) {
						addItemToTimeline (replyItem, divRootReplies);
						const divReplyThread = divRootReplies.children (".divThread").last ();
						if (replyItem.id === idLinked) {
							divLinkedThread = divReplyThread;
							}
						});
					const rootWedgeUI = divRootThread.data ("wedgeUI"); //7/9/26 by CC -- the root's replies just rendered underneath it; its wedge goes light
					if (rootWedgeUI !== undefined) {
						rootWedgeUI.paintWedge ();
						}
					openStoryArea ();
					if (divLinkedThread !== undefined) {
						selectThread (divLinkedThread, true); //cursor on the post the link pointed at, scrolled clear of the menu
						}
					}
				globals.myRssNetwork.getItemByGuid (guid, function (err, item) {
					if (err) {
						console.log ("loadStory: err.message == " + err.message);
						showStoryNotFound (err.message); //7/7/26 by CC -- the server's reason, e.g. the post has been deleted
						}
					else {
						if ((item === undefined) || (item.id === undefined)) { //6/21/26 by CC -- a not-found guid comes back from the server as an empty object, not undefined
							console.log ("loadStory: no post with guid == " + guid);
							showStoryNotFound ();
							}
						else {
							const idLinked = item.id;
							const idRoot = idLinked; //7/10/26 by CC -- a link to a reply used to open the parent's story with the reply nested inside (#121); now every post's link shows that post itself, with its own replies -- the Replying-to link is the way up, dive and surface cover the rest
							const screenname = globals.myRssNetwork.getScreenname ();
							globals.myRssNetwork.getItemAndReplies (screenname, idRoot, function (errContext, theItems) {
								if (errContext) {
									console.log ("loadStory: errContext.message == " + errContext.message);
									renderSingleStory (item);
									}
								else {
									var flHaveRoot = false;
									if (theItems !== undefined) {
										theItems.forEach (function (oneItem) {
											if (oneItem.id === idRoot) {
												flHaveRoot = true;
												}
											});
										}
									if (flHaveRoot === true) {
										renderStoryWithContext (theItems, idRoot, idLinked);
										}
									else {
										renderSingleStory (item); //the root went missing (e.g. a deleted parent) -- show the linked post on its own
										}
									}
								});
							}
						}
					});
				}

			function loadProfile (name) { //6/26/26 by CC -- #79: a user's profile page -- a header (name, @handle, description, self-linked URL) above their recent posts, rendered into its own area over the timeline, leaving the timeline untouched underneath
				function openProfileArea () {
					divTimeline.hide ();
					divStory.hide ();
					divProfile.show ();
					divChat.addClass ("profilePage");
					window.scrollTo (0, 0);
					}
				function showProfileNotFound (theMessage) {
					divProfile.empty ();
					divProfile.append ($('<div class="divProfileNotFound"></div>').text (theMessage));
					openProfileArea ();
					}
				globals.myRssNetwork.getUserData (name, function (err, theData) {
					if (err) {
						console.log ("loadProfile: err.message == " + err.message);
						showProfileNotFound ("Can't show the profile because there is no user named @" + name + ". :-)");
						}
					else {
						const prefs = theData.prefs || {};
						const displayName = prefs.myFeedTitle || name;

						divProfile.empty ();
						const divProfileHeader = $('<div class="divProfileHeader"></div>');
						divProfileHeader.append ($('<div class="divProfilePageName"></div>').text (displayName));
						divProfileHeader.append ($('<div class="divProfilePageHandle"></div>').text ("@" + name));
						if (prefs.myFeedDescription !== undefined) {
							divProfileHeader.append ($('<div class="divProfilePageDescription"></div>').text (prefs.myFeedDescription));
							}
						if (prefs.myFeedLink !== undefined) {
							divProfileHeader.append ($('<a class="aProfilePageLink" target="_blank"></a>').attr ("href", prefs.myFeedLink).text (prefs.myFeedLink));
							}
						divProfile.append (divProfileHeader);

						const divProfileTimeline = $('<div class="divProfileTimeline"></div>');
						divProfile.append (divProfileTimeline);
						openProfileArea ();

						globals.myRssNetwork.getRecentUserItems (name, function (err, theItems) {
							if (err) {
								console.log ("loadProfile: err.message == " + err.message);
								}
							else {
								theItems.forEach (function (item) { //newest first from the server; append keeps newest at top
									addItemToTimeline (item, divProfileTimeline);
									});
								}
							});
						}
					});
				}

			divNav.append (divNavHome);
			divNav.append (divNavCompose);

			if (options.editorMenu !== undefined) { //6/10/26 by Claude -- menu entries come from the structure the app passes in
				fillMenuPopup (divContextMenuPopup, options.editorMenu);
				}
			divContextMenu.append (buttonContextMenu);
			divContextMenu.append (divContextMenuPopup);
			divModalToolbar.append (buttonModalBold);
			divModalToolbar.append (buttonModalItalic);
			divModalToolbar.append (buttonModalLink);
			divModalToolbar.append (buttonModalBulletList);
			divModalToolbar.append (buttonModalNumberList);
			divModalToolbar.append (buttonModalMarkdown);
			divModalToolbar.append (divContextMenu);

			divComposerBody.append (divComposerToolbar);
			divComposerBody.append (inputComposer);

			divComposer.append (divComposerAvatar);
			divComposer.append (divComposerBody);
			divComposer.append (buttonPost);

			divChatCenter.append (divCenterHeader);
			divChatCenter.append (divComposer);
			divChatCenter.append (divTimeline);
			divChatCenter.append (divStory);
			divChatCenter.append (divProfile);
			divChatCenter.append (divScanner);

			divUsersCard.append ($('<div class="divCardTitle">All users</div>'));
			divUsersCard.append ($('<a class="aUsersLink" target="_blank">Browse subscription list</a>').attr ("href", "/getsubscriptionlist"));

			function addToolTip (theObject, tipText, placement) { //6/7/26 by Claude -- ported from wordland misc.js
				const flOnTouchDevice = ("ontouchstart" in window) || (navigator.maxTouchPoints > 0); //no hovering on tablets and phones
				if (flOnTouchDevice === false) {
					theObject.on ("click", function () {
						$(this).tooltip ("hide");
						});
					theObject.tooltip ({
						title: tipText,
						placement: placement,
						container: "body",
						trigger: "hover focus",
						delay: {show: 500, hide: 200} //extra time to get pointer onto tooltip
						});
					}
				}

			function renderIconBar () {
				if (options.iconBar === undefined) {
					return;
					}
				const divIconsContainer = $('<div class="divIconsContainer"></div>');
				options.iconBar.forEach (function (item) {
					const divIcon = $('<div class="divIcon"></div>');
					item.theIcon = divIcon;
					divIcon.data ("iconDef", item); //6/1/26 by Claude -- per-second refresh reads enabled from here
					divIcon.append (item.icon);
					const spanIconLabel = $('<span class="spanIconLabel"></span>').text ((item.title !== undefined) ? item.title : item.name); //7/9/26 by CC -- the panel look: each icon carries a short label. 7/10/26 by CC -- #168: the label comes from the icon rec's own title field now, not a theme-side mapping
					divIcon.append (spanIconLabel);
					if (item.tooltip !== undefined) {
						addToolTip (divIcon, item.tooltip, "bottom"); //7/16/26 by CC -- was "right", which floated the tip out over the timeline; below the row it stays with the icon and its word
						}
					divIcon.on ("click", function (ev) {
						if (item.enabled !== true) { //disabled icons ignore clicks
							return;
							}
						if (typeof item.click === "function") {
							item.click (ev);
							}
						});
					divIconsContainer.append (divIcon);
					});
				divChatLeft.append (divIconsContainer);
				divChatLeft.append (divMapOutline); //7/9/26 by CC -- the map lives under the icon list, like Recents in the Claude sidebar
				}
			renderIconBar ();

			const divThemeVersion = $('<div class="divThemeVersion">v' + themesVersion + '</div>'); //6/10/26 by Claude -- was hardcoded at v0.5.152, now shows the real version
			divChatRight.append (divThemeVersion);

			divChat.append (divChatLeft);
			divChat.append (divChatCenter);
			divChat.append (divChatRight);
			options.whereToAppend.append (divChat);

			refreshIconsEnabled (); //6/1/26 by Claude -- initial paint once the icons are assembled into the page, then keep it mirrored every second
			setInterval (refreshIconsEnabled, 1000);

			divReplyModalTop.append (divComposerTitle);
			divReplyModalTop.append (divModalToolbar);
			divReplyParentBody.append (divReplyParentAuthor);
			divReplyParentBody.append (divReplyParentSummary); //7/20/26 by CC -- #155: the wedge line sits directly under the author's name
			divReplyParentBody.append (divReplyParentTitle);
			divReplyParentBody.append (divReplyParentText);
			divReplyParent.append (divReplyParentAvatar);
			divReplyParent.append (divReplyParentBody);
			divReplyComposerRow.append (divReplyComposerAvatar);
			divReplyComposerRow.append (inputReplyComposer);
			divReplyComposerRow.append (textareaMarkdown); //7/5/26 by CC -- #107: the markdown text box sits in the same spot; setEditorMode shows one surface at a time
			textareaMarkdown.hide ();
			const divModalStatusGroup = $('<div class="divModalStatusGroup"></div>');
			divModalStatusGroup.append (divModalStatus);
			divModalStatusGroup.append (divModalWhen);
			divModalStatusGroup.append (divModalCharCount);

			const divModalButtonGroup = $('<div class="divModalButtonGroup"></div>');
			divModalButtonGroup.append (buttonReplyCancel);
			divModalButtonGroup.append (buttonReplyPost);

			divModalBottom.append (divModalStatusGroup);
			divModalBottom.append (divModalButtonGroup);
			divReplyModal.append (divReplyParent);
			divReplyModal.append (divReplyParentDivider);
			divReplyModal.append (divReplyModalTop);
			divReplyModal.append ($('<div class="divReplyDividerLine"></div>'));
			divReplyModal.append (divReplyComposerRow);
			divReplyModal.append (divModalBottom);
			divReplyOverlay.append (divReplyModal);
			$(document.body).append (divReplyOverlay);
			divReplyOverlay.hide ();

			inputComposer.on ("mousedown", function (event) {
				event.preventDefault ();
				openEditWindow ();
				});
			$(document).on ("keydown", function (event) {
				if (event.key !== "ArrowUp" && event.key !== "ArrowDown") {
					return;
					}
				const active = document.activeElement;
				if (active !== null) {
					if (active.isContentEditable) {
						return;
						}
					if (active.tagName === "INPUT" || active.tagName === "TEXTAREA") {
						return;
						}
					}
				var allThreads; //6/27/26 by CC -- #104: flatup/flatdown drives whichever list is on screen -- the flat scanner when it's open, otherwise the timeline
				if (divChat.hasClass ("scannerPage") === true) {
					allThreads = divScanner.find (".divThread"); //7/4/26 by CC -- #140: the structure view nests replies; walk every node in document order, same as the timeline
					}
				else {
					allThreads = divTimeline.find (".divThread"); //6/26/26 by CC -- every item in document order, replies included, so up/down steps through the whole outline, not just top-level posts
					}
				if (allThreads.length === 0) {
					return;
					}
				event.preventDefault ();
				var currentIndex = -1;
				if (divSelectedThread !== undefined) {
					currentIndex = allThreads.index (divSelectedThread);
					}
				var nextIndex;
				if (event.key === "ArrowDown") {
					nextIndex = currentIndex + 1;
					if (nextIndex >= allThreads.length) {
						nextIndex = allThreads.length - 1;
						}
					}
				else {
					nextIndex = currentIndex - 1;
					if (nextIndex < 0) {
						nextIndex = 0;
						}
					}
				if (nextIndex === currentIndex) {
					return;
					}
				selectThread ($(allThreads [nextIndex]), true);
				});
			buttonPost.on ("click", openEditWindow);
			divNavCompose.on ("click", openEditWindow);

			function execToolbarCommand (event, command) {
				event.preventDefault ();
				inputComposer.focus ();
				document.execCommand (command);
				saveDraft ();
				}
			buttonBold.on ("mousedown", function (event) {
				execToolbarCommand (event, "bold");
				});
			buttonItalic.on ("mousedown", function (event) {
				execToolbarCommand (event, "italic");
				});
			buttonLink.on ("mousedown", function (event) {
				event.preventDefault ();
				const sel = window.getSelection (); //6/24/26 by CC -- save the selection before the dialog steals focus, restore it before linking, so the link wraps the selected text instead of landing at the start
				const savedRange = (sel.rangeCount > 0) ? sel.getRangeAt (0).cloneRange () : undefined;
				askDialog ("Enter URL for link:", appPrefs.lastUrlString || "", "https://", function (url, flCancelled) {
					if (!flCancelled && url) {
						appPrefs.lastUrlString = url;
						prefsChanged ();
						inputComposer.focus ();
						if (savedRange !== undefined) {
							const selNow = window.getSelection ();
							selNow.removeAllRanges ();
							selNow.addRange (savedRange);
							}
						document.execCommand ("createLink", false, url);
						saveDraft ();
						}
					});
				});
			function updateLinkButtons () { //6/24/26 by CC -- a Link button is enabled only when there's a non-empty selection inside its own editor
				function selectionInside (theComposer) {
					var flInside = false;
					const sel = window.getSelection ();
					if (sel.rangeCount > 0) {
						const range = sel.getRangeAt (0);
						if (range.collapsed === false) {
							if (theComposer [0].contains (range.commonAncestorContainer) === true) {
								flInside = true;
								}
							}
						}
					return (flInside);
					}
				buttonLink.prop ("disabled", selectionInside (inputComposer) === false);
				if (currentEditorMode === "markdown") { //7/5/26 by CC -- #107: the text box tracks its own selection
					const theTextarea = textareaMarkdown [0];
					buttonModalLink.prop ("disabled", theTextarea.selectionStart === theTextarea.selectionEnd);
					}
				else {
					buttonModalLink.prop ("disabled", selectionInside (inputReplyComposer) === false);
					}
				}
			$(document).on ("selectionchange", updateLinkButtons);
			updateLinkButtons ();

			buttonBulletList.on ("mousedown", function (event) {
				execToolbarCommand (event, "insertUnorderedList");
				});
			buttonNumberList.on ("mousedown", function (event) {
				execToolbarCommand (event, "insertOrderedList");
				});

			function execModalCommand (event, command) {
				event.preventDefault ();
				inputReplyComposer.focus ();
				document.execCommand (command);
				}
			buttonModalBold.on ("mousedown", function (event) {
				event.preventDefault ();
				if (currentEditorMode === "markdown") {
					wrapSelectionMarkdown ("**", "**");
					}
				else {
					execModalCommand (event, "bold");
					}
				});
			buttonModalItalic.on ("mousedown", function (event) {
				event.preventDefault ();
				if (currentEditorMode === "markdown") {
					wrapSelectionMarkdown ("*", "*");
					}
				else {
					execModalCommand (event, "italic");
					}
				});
			buttonModalLink.on ("mousedown", function (event) {
				event.preventDefault ();
				const sel = window.getSelection (); //6/24/26 by CC -- save the selection before the dialog steals focus, restore it before linking, so the link wraps the selected text instead of landing at the start
				const savedRange = (sel.rangeCount > 0) ? sel.getRangeAt (0).cloneRange () : undefined;
				askDialog ("Enter URL for link:", appPrefs.lastUrlString || "", "https://", function (url, flCancelled) {
					if (!flCancelled && url) {
						appPrefs.lastUrlString = url;
						prefsChanged ();
						if (currentEditorMode === "markdown") { //7/5/26 by CC -- #107: the text box remembers its own selection through the dialog; wrapSelectionMarkdown focuses it
							wrapSelectionMarkdown ("[", "](" + url + ")");
							}
						else {
							inputReplyComposer.focus ();
							if (savedRange !== undefined) {
								const selNow = window.getSelection ();
								selNow.removeAllRanges ();
								selNow.addRange (savedRange);
								}
							document.execCommand ("createLink", false, url);
							}
						}
					});
				});
			buttonModalBulletList.on ("mousedown", function (event) {
				event.preventDefault ();
				if (currentEditorMode === "markdown") {
					wrapSelectionMarkdown ("- ", "");
					}
				else {
					execModalCommand (event, "insertUnorderedList");
					}
				});
			buttonModalNumberList.on ("mousedown", function (event) {
				event.preventDefault ();
				if (currentEditorMode === "markdown") {
					wrapSelectionMarkdown ("1. ", "");
					}
				else {
					execModalCommand (event, "insertOrderedList");
					}
				});
			buttonModalMarkdown.on ("mousedown", function (event) {
				event.preventDefault ();
				toggleMarkdownMode ();
				});

			divModalCharCount.on ("click", function () {
				appPrefs.flWordCount = !appPrefs.flWordCount;
				prefsChanged ();
				updateModalCharCount ();
				});

			buttonContextMenu.on ("mousedown", function (event) {
				event.preventDefault ();
				});
			buttonContextMenu.on ("click", function (event) {
				event.stopPropagation ();
				const flWasOpen = divContextMenuPopup.hasClass ("flOpen");
				$(".divContextMenuPopup").removeClass ("flOpen");
				if (!flWasOpen) {
					refreshIconsEnabled (); //6/10/26 by Claude -- mirror enabled state right now, so the menu opens correct instead of waiting for the next tick
					divContextMenuPopup.addClass ("flOpen");
					}
				});
			function refreshComposerTitle () {
				if (currentTitle === undefined) {
					divComposerTitle.text ("Title?").addClass ("flPlaceholder");
					}
				else {
					divComposerTitle.text (currentTitle).removeClass ("flPlaceholder");
					}
				divComposerTitle.show ();
				}
			function refreshUpdateDisabled () { //5/24/26 by Claude -- Update button enabled only when title or body differs from the item being edited
				if (editTargetItem === undefined) {
					return;
					}
				const flBodyEmpty = getEditorText ().trim ().length === 0;
				var flBodyChanged; //7/5/26 by CC -- #107: compare in the language of the surface being edited
				if (currentEditorMode === "markdown") {
					flBodyChanged = (textareaMarkdown.val () !== editOriginalMd);
					}
				else {
					flBodyChanged = (inputReplyComposer.html () !== editOriginalBody);
					}
				const flTitleChanged = currentTitle !== editOriginalTitle;
				const flDisabled = flBodyEmpty || (!flBodyChanged && !flTitleChanged) || (ctPendingImageUploads > 0); //7/22/26 by CC -- #188: no publishing while an image is still uploading
				buttonReplyPost.prop ("disabled", flDisabled);
				}
			function setTitleDialog () {
				const theDefault = currentTitle !== undefined ? currentTitle : (appPrefs.lastTitleString || "");
				askDialog ("Title for this post:", theDefault, "", function (response, flCancelled) {
					if (flCancelled) {
						return;
						}
					if (response.length === 0) {
						currentTitle = undefined;
						}
					else {
						currentTitle = response;
						appPrefs.lastTitleString = response;
						prefsChanged ();
						}
					refreshComposerTitle ();
					saveTitle ();
					refreshUpdateDisabled (); //5/24/26 by Claude
					});
				}
			function startTitleEdit () { //5/30/26 by Claude -- edit the title in place instead of in a dialog
				const startText = (currentTitle !== undefined) ? currentTitle : "";
				divComposerTitle.text (startText).removeClass ("flPlaceholder");
				divComposerTitle.attr ("contenteditable", "true");
				divComposerTitle.focus ();

				const theSelection = window.getSelection (); //put the cursor at the end of the text
				const theRange = document.createRange ();
				theRange.selectNodeContents (divComposerTitle [0]);
				theRange.collapse (false);
				theSelection.removeAllRanges ();
				theSelection.addRange (theRange);
				}
			function finishTitleEdit () { //5/30/26 by Claude -- commit the in-place edit; no cancel by design
				divComposerTitle.attr ("contenteditable", "false");
				const newTitle = divComposerTitle.text ().trim ();
				if (newTitle.length === 0) {
					currentTitle = undefined;
					}
				else {
					currentTitle = newTitle;
					appPrefs.lastTitleString = newTitle;
					prefsChanged ();
					}
				refreshComposerTitle ();
				saveTitle ();
				refreshUpdateDisabled ();
				}
			const savedTitle = localStorage.getItem (options.titleKey);
			if (savedTitle !== null) {
				currentTitle = savedTitle;
				}
			refreshComposerTitle ();
			divComposerTitle.on ("click", function () {
				if (divComposerTitle.attr ("contenteditable") === "true") { //already editing -- let the click place the cursor
					return;
					}
				startTitleEdit ();
				});
			divComposerTitle.on ("blur", function () {
				if (divComposerTitle.attr ("contenteditable") === "true") {
					finishTitleEdit ();
					}
				});
			divComposerTitle.on ("keydown", function (ev) {
				if (ev.key === "Enter") { //commit on Enter, a title is a single line
					ev.preventDefault ();
					divComposerTitle.blur ();
					}
				});
			divComposerTitle.on ("input", function () { //5/30/26 by Claude -- live-enable the publish/update button as the title changes
				const liveTitle = divComposerTitle.text ().trim ();
				currentTitle = (liveTitle.length === 0) ? undefined : liveTitle;
				if (editTargetItem !== undefined) {
					refreshUpdateDisabled ();
					}
				else {
					buttonReplyPost.prop ("disabled", getEditorText ().trim ().length === 0); //7/5/26 by CC -- #107: mode-aware
					}
				});
			function getEditorData () { //6/10/26 by Claude -- the editor's current state, for the app's view-data command
				const theData = {
					description: (currentEditorMode === "markdown") ? getHtmlFromMarkdown (textareaMarkdown.val ()) : inputReplyComposer.html (), //7/5/26 by CC -- #107
					markdowntext: (currentEditorMode === "markdown") ? textareaMarkdown.val () : mdSourceText,
					title: currentTitle,
					mode: currentEditorMode,
					id: (editTargetItem !== undefined) ? editTargetItem.id : undefined,
					guid: (editTargetItem !== undefined) ? editTargetItem.guid : undefined,
					feedUrl: globals.myRssNetwork.getFeedUrl (),
					author: globals.myRssNetwork.getScreenname (),
					pubDate: (editTargetItem !== undefined) ? editTargetItem.pubDate : undefined,
					inReplyTo: (editTargetItem !== undefined) ? editTargetItem.inReplyTo : ((replyTargetItem !== undefined) ? replyTargetItem.id : undefined),
					replyingTo: replyTargetItem,
					flDraftChanged: flDraftChanged,
					savedUserDraft: appPrefs.savedUserDraft
					};
				return theData;
				}
			$(document).on ("mousedown", function (event) {
				if ($(event.target).closest (".divContextMenu").length === 0) {
					$(".divContextMenuPopup").removeClass ("flOpen");
					}
				if (($(event.target).closest (".divThread").length === 0) && ($(event.target).closest (".divIconsContainer").length === 0)) { //6/27/26 by CC -- #104: don't drop the cursor when the click is on a left-bar icon -- those icons (hoist/dehoist/etc.) act on the selected post, so the selection must survive the click
					deselectThread ();
					}
				});

			buttonReplyCancel.on ("click", closeReplyModal);
			var flPressStartedOnOverlay = false; //7/5/26 by CC -- #138: a selection swipe that starts inside the editor and ends outside it fires a click on the overlay; only a press that STARTED on the overlay is a real click-away
			divReplyOverlay.on ("mousedown", function (event) {
				flPressStartedOnOverlay = (event.target === divReplyOverlay [0]);
				});
			divReplyOverlay.on ("click", function (event) {
				if ((event.target === divReplyOverlay [0]) && (flPressStartedOnOverlay === true)) {
					closeReplyModal ();
					}
				});
			buttonReplyPost.on ("click", function () {
				var htmlText, markdownText;
				if (currentEditorMode === "markdown") {
					markdownText = textareaMarkdown.val (); //7/5/26 by CC -- #107: the text box holds the document
					htmlText = getHtmlFromMarkdown (markdownText);
					}
				else {
					htmlText = inputReplyComposer.html ();
					markdownText = mdSourceText; //7/5/26 by CC -- #107: kept current by the wizzy input handler
					}
				const text = getEditorText ().trim ();
				if (text.length === 0) {
					return;
					}
				if (editTargetItem !== undefined) {
					updateMessageFromModal (htmlText, markdownText);
					}
				else {
					if (replyTargetItem !== undefined) {
						postReply (replyTargetItem.id, htmlText, markdownText);
						}
					else {
						sendMessageFromModal (htmlText, markdownText);
						}
					}
				});
			inputReplyComposer.on ("mousedown keydown", function () {
				inputReplyComposer.addClass ("flUserTouched");
				});
			inputReplyComposer.on ("keydown", function (event) { //5/23/26 by Claude
				if (event.key !== "Enter") {
					return;
					}
				const flMarkdown = (currentEditorMode === "markdown");
				if (event.shiftKey) {
					event.preventDefault ();
					if (flMarkdown) {
						document.execCommand ("insertText", false, "\n");
						}
					else {
						document.execCommand ("insertLineBreak");
						}
					return;
					}
				if (flMarkdown) {
					event.preventDefault ();
					document.execCommand ("insertText", false, "\n\n");
					return;
					}

				const editor = inputReplyComposer [0];

				//7/23/26 by CC -- pasted inline fragments (text with a link, no paragraph wrapper) sit as bare children of the editor; wrapping each text node in its own paragraph stranded inline elements between blocks and evicted the caret, so the first Return after such a paste broke the text at the link instead of the cursor. Wrap each contiguous inline run in ONE paragraph, then put the caret back before the browser splits.
				const theSelection = window.getSelection ();
				var savedCaretNode, savedCaretOffset;
				if ((theSelection !== null) && (theSelection.rangeCount > 0)) {
					savedCaretNode = theSelection.anchorNode;
					savedCaretOffset = theSelection.anchorOffset;
					}
				const inlineTags = {A: true, B: true, STRONG: true, I: true, EM: true, U: true, CODE: true, IMG: true, SPAN: true, BR: true};
				var currentWrapper = null, flWrappedSomething = false;
				Array.from (editor.childNodes).forEach (function (node) {
					const flInline = (node.nodeType === 3) || ((node.nodeType === 1) && (inlineTags [node.tagName] === true));
					if (flInline) {
						if (currentWrapper === null) {
							currentWrapper = document.createElement ("p");
							editor.insertBefore (currentWrapper, node);
							}
						currentWrapper.appendChild (node);
						flWrappedSomething = true;
						}
					else {
						currentWrapper = null;
						}
					});
				if (flWrappedSomething && (savedCaretNode !== undefined) && (savedCaretNode !== editor)) {
					const newRange = document.createRange ();
					newRange.setStart (savedCaretNode, savedCaretOffset);
					newRange.collapse (true);
					theSelection.removeAllRanges ();
					theSelection.addRange (newRange);
					}
				});
			function escapePastedText (theText) {
				return theText.replace (/&/g, "&amp;").replace (/</g, "&lt;").replace (/>/g, "&gt;");
				}
			function plainTextToBlockHtml (theText) {
				const paragraphs = theText.split (/\n\n+/);
				var result = "";
				paragraphs.forEach (function (para) {
					const lines = para.split ("\n");
					var paraHtml = "";
					lines.forEach (function (line, ix) {
						if (ix > 0) {
							paraHtml += "<br>";
							}
						paraHtml += escapePastedText (line);
						});
					if (paraHtml.length === 0) { //empty paragraph -- preserve as a blank block
						paraHtml = "<br>";
						}
					result += "<div>" + paraHtml + "</div>";
					});
				return result;
				}
			function sanitizePastedHtml (htmlSource) {
				const allowedTags = {B: true, STRONG: true, I: true, EM: true, U: true, A: true, BR: true, P: true, DIV: true, UL: true, OL: true, LI: true, CODE: true, PRE: true, BLOCKQUOTE: true, H1: true, H2: true, H3: true, H4: true, H5: true, H6: true};
				const doc = new DOMParser ().parseFromString (htmlSource, "text/html");
				function cleanNode (node) {
					const children = Array.from (node.childNodes);
					children.forEach (function (child) {
						if (child.nodeType === 1) { //element
							cleanNode (child);
							if (allowedTags [child.tagName] === true) {
								const attrs = Array.from (child.attributes);
								attrs.forEach (function (attr) {
									if (child.tagName === "A" && attr.name === "href") {
										//keep href on links
										}
									else {
										child.removeAttribute (attr.name);
										}
									});
								if (child.tagName === "A") {
									child.setAttribute ("target", "_blank");
									child.setAttribute ("rel", "noopener noreferrer");
									}
								}
							else { //disallowed -- unwrap, keeping children
								while (child.firstChild !== null) {
									child.parentNode.insertBefore (child.firstChild, child);
									}
								child.parentNode.removeChild (child);
								}
							}
						});
					}
				cleanNode (doc.body);
				return doc.body.innerHTML;
				}
			function uploadPastedImage (theFile) { //7/22/26 by CC -- #188: a pasted image uploads to the server, the picture appears at the cursor
				const maxImageBytes = 2 * 1024 * 1024; //matches the server's default maxMediaUploadBytes
				if (globals.myRssNetwork.uploadMedia === undefined) {
					alertDialog ("Can't add the image because the software isn't up to date.");
					return;
					}
				if (theFile.size > maxImageBytes) {
					alertDialog ("Can't add the image because it's larger than the 2MB limit.");
					return;
					}
				const idPlaceholder = "idImageUpload" + idNextImageUpload++;
				document.execCommand ("insertHTML", false, '<span class="spanImageUploading" id="' + idPlaceholder + '">Uploading image&#x2026;</span>'); //horizontal ellipsis
				ctPendingImageUploads++;
				editorContentChanged ();
				const theReader = new FileReader ();
				theReader.onload = function () {
					const base64text = theReader.result.substring (theReader.result.indexOf (",") + 1);
					globals.myRssNetwork.uploadMedia (theFile.type, base64text, function (err, data) {
						ctPendingImageUploads--;
						const spanPlaceholder = $("#" + idPlaceholder);
						if (err) {
							spanPlaceholder.remove ();
							alertDialog (err.message);
							}
						else {
							spanPlaceholder.replaceWith ('<img src="' + data.url + '">');
							}
						editorContentChanged ();
						scheduleDraftSave ();
						});
					};
				theReader.readAsDataURL (theFile);
				}
			inputReplyComposer.on ("paste", function (event) {
				event.preventDefault ();
				const clipboardData = event.originalEvent.clipboardData || window.clipboardData;
				if (clipboardData === undefined || clipboardData === null) {
					return;
					}
				const flMarkdownMode = inputReplyComposer.hasClass ("flMarkdownMode");

				const theClipboardItems = clipboardData.items; //7/22/26 by CC -- #188: a pasted image (screenshot or copied image file) uploads and lands at the cursor
				if ((theClipboardItems !== undefined) && (!flMarkdownMode)) {
					var flFoundImage = false;
					for (var i = 0; i < theClipboardItems.length; i++) {
						if ((theClipboardItems [i].kind === "file") && (theClipboardItems [i].type.indexOf ("image/") === 0)) {
							uploadPastedImage (theClipboardItems [i].getAsFile ());
							flFoundImage = true;
							}
						}
					if (flFoundImage) {
						return;
						}
					}

				const clipboardPlainText = clipboardData.getData ("text/plain");
				const trimmedClipboard = clipboardPlainText.trim ();
				const flClipboardIsUrl = /^https?:\/\/\S+$/.test (trimmedClipboard); //5/26/26 by Claude -- cute paste (escapes doubled for outer template literal)
				const theSelection = window.getSelection ();
				const selectedText = (theSelection !== null) ? theSelection.toString () : "";
				const flCutePaste = flClipboardIsUrl && selectedText.length > 0;

				if (flMarkdownMode === true) {
					if (flCutePaste === true) {
						const markdownLink = "[" + selectedText + "](" + trimmedClipboard + ")";
						document.execCommand ("insertText", false, markdownLink);
						return;
						}
					document.execCommand ("insertText", false, clipboardPlainText);
					return;
					}
				if (flCutePaste === true) {
					const safeUrl = trimmedClipboard.replace (/"/g, "&quot;");
					const safeText = selectedText.replace (/&/g, "&amp;").replace (/</g, "&lt;").replace (/>/g, "&gt;");
					const anchorHtml = '<a href="' + safeUrl + '" target="_blank" rel="noopener noreferrer">' + safeText + '</a>';
					document.execCommand ("insertHTML", false, anchorHtml);
					return;
					}
				const htmlSource = clipboardData.getData ("text/html");
				var cleanHtml;
				if (htmlSource !== undefined && htmlSource.length > 0) {
					cleanHtml = sanitizePastedHtml (htmlSource);
					}
				else {
					const plainText = clipboardData.getData ("text/plain");
					const flLooksLikeHtml = /<[a-zA-Z]/.test (plainText); //detect HTML-shaped text -- a letter immediately following an opening angle bracket
					if (flLooksLikeHtml === true) {
						cleanHtml = sanitizePastedHtml (plainText);
						}
					else {
						cleanHtml = plainTextToBlockHtml (plainText);
						}
					}
				document.execCommand ("insertHTML", false, cleanHtml);
				});
			textareaMarkdown.on ("paste", function (event) { //7/5/26 by CC -- #107: a text box pastes plain text natively; the only special case is cute paste -- a copied URL over selected text becomes a markdown link
				const clipboardData = event.originalEvent.clipboardData || window.clipboardData;
				if (clipboardData === undefined || clipboardData === null) {
					return;
					}
				const trimmedClipboard = clipboardData.getData ("text/plain").trim ();
				const flClipboardIsUrl = /^https?:\/\/\S+$/.test (trimmedClipboard);
				const theTextarea = textareaMarkdown [0];
				if (flClipboardIsUrl && (theTextarea.selectionStart !== theTextarea.selectionEnd)) {
					event.preventDefault ();
					const startPos = theTextarea.selectionStart;
					const endPos = theTextarea.selectionEnd;
					const theText = theTextarea.value;
					const selectedText = theText.substring (startPos, endPos);
					const markdownLink = "[" + selectedText + "](" + trimmedClipboard + ")";
					theTextarea.value = theText.substring (0, startPos) + markdownLink + theText.substring (endPos);
					const caretPos = startPos + markdownLink.length;
					theTextarea.setSelectionRange (caretPos, caretPos);
					textareaMarkdown.trigger ("input");
					}
				});
			function scheduleDraftSave () { //7/5/26 by CC -- #107: factored from the input handler so the mode flip can save too; the draft remembers which mode you were in
				flDraftChanged = true; //6/21/26 by CC -- auto-save the draft in every mode (new, reply, edit), not just new posts
				updateModalStatus ();
				clearTimeout (draftSaveTimeout);
				draftSaveTimeout = setTimeout (function () {
					saveDraft ();
					flDraftChanged = false;
					updateModalStatus ();
					}, 1000);
				}
			function editorContentChanged () { //7/5/26 by CC -- #107: everything a keystroke triggers, shared by both surfaces
				divComposerTitle.tooltip ("hide"); //#49 -- typing in the body hides the title prompt tooltip so it can't overlap the text
				const flEmpty = getEditorText ().trim ().length === 0;
				if (flEmpty === true) {
					inputReplyComposer.removeClass ("flUserTouched");
					}
				if (editTargetItem !== undefined) { //5/24/26 by Claude -- in edit mode Update is gated on actual change, not just non-empty
					refreshUpdateDisabled ();
					}
				else {
					buttonReplyPost.prop ("disabled", flEmpty || (ctPendingImageUploads > 0)); //7/22/26 by CC -- #188: no publishing while an image is still uploading
					}
				scheduleDraftSave ();
				updateModalCharCount ();
				}
			inputReplyComposer.on ("input", function () {
				mdSourceText = getMarkdownFromHtml (inputReplyComposer.html ()); //7/5/26 by CC -- #107: every wizzy edit updates the markdown document, wordland-style; an untouched render never does
				editorContentChanged ();
				});
			textareaMarkdown.on ("input", function () {
				mdSourceText = textareaMarkdown.val (); //7/5/26 by CC -- #107: the text box IS the document
				editorContentChanged ();
				});
			$(document).on ("keydown", function (event) {
				if (event.key === "Escape" && divReplyOverlay.is (":visible")) {
					closeReplyModal ();
					}
				});

			updateForLogin ();
			if (options.profileName !== undefined) { //6/26/26 by CC -- #79: cold-load on ?name= opens that profile
				loadProfile (options.profileName);
				}
			else {
				if (options.storyGuid === undefined) {
					loadRecentItems ();
					}
				else {
					loadStory (options.storyGuid);
					}
				}

			this.newItem = function (item) {
				if (item.guid !== undefined && itemsByGuid [item.guid] !== undefined) { //6/18/26 by Claude -- already on screen (e.g. our own just-published post, shown immediately); refresh it with the authoritative server copy (real name + avatar) instead of dropping it, so a later redraw (hoist) doesn't fall back to the default avatar -- 6/28/26 by CC
					updateDisplayedItem (item);
					return;
					}
				if (item.guid !== undefined && pendingReplyMap [item.guid] !== undefined) {
					const pending = pendingReplyMap [item.guid];
					item.inReplyTo = pending.inReplyTo;
					item.id = pending.id;
					delete pendingReplyMap [item.guid];
					}
				addItemToTimeline (item); //6/25/26 by CC -- addItemToTimeline now does the inReplyToNum -> inReplyTo mapping for every render path
				bumpParentReplyCount (item); //7/3/26 by CC -- a live reply from the socket is newer than the parent's server count
				};
			function updateDisplayedItem (chatItem) { //5/22/26 by Claude -- update a displayed post in place -- from the socket, or from our own just-saved edit
				const entry = itemsByGuid [chatItem.guid];
				if (entry === undefined) { //item not displayed; nothing to update
					return;
					}
				if (chatItem.inReplyTo === undefined && chatItem.inReplyToNum !== undefined) { //7/3/26 by CC -- same mapping addItemToTimeline does; without it this replace wiped entry.item.inReplyTo, so a later delete skipped the parent's badge and thread scans lost the reply
					chatItem.inReplyTo = chatItem.inReplyToNum;
					}
				entry.item = chatItem;

				if (chatItem.ctLikes !== undefined) { //6/25/26 by CC -- #75: keep the like count live when anyone likes or unlikes (the broadcast carries the true count). 7/8/26 by CC -- the -1 bug: the count now reaches EVERY rendered copy, not just the timeline's
					if (entry.spanLikeCount !== undefined) {
						setLikeCountDisplay (entry, chatItem.ctLikes);
						}
					eachRenderedHeart (chatItem.id, function (likeUI) {
						setLikeCountDisplay (likeUI, chatItem.ctLikes);
						});
					}

				if (chatItem.ctReplies !== undefined && entry.spanReplyCount !== undefined) { //7/3/26 by CC -- the broadcast carries the true reply count; take it
					entry.replyCount = chatItem.ctReplies;
					entry.spanReplyCount.text (chatItem.ctReplies);
					if (entry.paintWedge !== undefined) { //7/9/26 by CC -- the count changed; the wedge may change shade
						entry.paintWedge ();
						}
					}

				//6/2/26 by Claude -- avatar is feed-level; repaint it on every displayed post from the same feed
				if (chatItem.imageUrl !== undefined) { //6/25/26 by CC -- #75: a like broadcast carries no avatar; don't repaint avatars to the default when imageUrl is absent
					Object.keys (itemsById).forEach (function (id) {
						const feedEntry = itemsById [id];
						if (feedEntry.item.feedUrl === chatItem.feedUrl) {
							feedEntry.item.imageUrl = chatItem.imageUrl;
							const feedAvatar = feedEntry.divThread.find (".divAvatar").first ();
							const feedAuthor = feedEntry.item.author || "?";
							const feedInitial = feedAuthor.charAt (0).toUpperCase ();
							populateAvatar (feedAvatar, chatItem.imageUrl || appConsts.urlDefaultImage, feedInitial);
							}
						});
					}

				const divTweetText = entry.divThread.find (".divTweetText").first ();
				divTweetText.html (chatItem.description);
				entry.refreshAfterUpdate (); //7/5/26 by CC -- #153: was expandFully; keep the reader's collapsed state so the icons don't jump when a like or edit updates the post
				const divTweetTitle = entry.divThread.find (".divTweetTitle").first ();
				const flHasTitle = (chatItem.title !== undefined && chatItem.title.length > 0);
				if (flHasTitle === true) {
					if (divTweetTitle.length === 0) { //title appeared on an item that had none -- insert before body
						const divNewTitle = $('<div class="divTweetTitle"></div>');
						fillTitleDiv (divNewTitle, chatItem.title, chatItem.link, chatItem.guid);
						divTweetText.before (divNewTitle);
						}
					else {
						fillTitleDiv (divTweetTitle, chatItem.title, chatItem.link, chatItem.guid);
						}
					}
				else {
					if (divTweetTitle.length > 0) { //title removed
						divTweetTitle.remove ();
						}
					}
				}
			this.updateItem = updateDisplayedItem;
			this.removeItem = function (item) { //6/12/26 by Claude -- delete confirmed by server; drop the post, promote its direct replies to top level
				const entry = itemsById [item.id];
				if (entry !== undefined) { //7/5/26 by CC -- #120: was an early return; a post deleted from a cold-loaded story page isn't in the maps, but the landing below still applies
					const flWasTopLevel = (entry.divThread.parent () [0] === divTimeline [0]);
					const replyThreads = entry.divReplies.children (".divThread");
					replyThreads.each (function () {
						const replyThread = $(this);
						const replyEntry = itemsById [replyThread.data ("itemId")];
						replyThread.children (".divTweet").find (".divReplyingTo").remove (); //no longer subordinate to anything
						if (replyEntry !== undefined) {
							replyEntry.item.inReplyTo = undefined;
							}
						attachThreadClick (replyThread);
						});
					if (flWasTopLevel) { //replies take the deleted post's spot in the timeline
						entry.divThread.before (replyThreads);
						}
					else { //the deleted post was itself a reply -- its replies rise to the top of the timeline
						divTimeline.prepend (replyThreads);
						}
					if (entry.item.inReplyTo !== undefined) { //deleted post was a reply -- drop its parent's reply count
						const parentEntry = itemsById [entry.item.inReplyTo];
						if (parentEntry !== undefined) {
							parentEntry.replyCount = parentEntry.replyCount - 1;
							parentEntry.spanReplyCount.text (parentEntry.replyCount);
							if (parentEntry.paintWedge !== undefined) { //7/9/26 by CC -- the parent just lost a reply; its wedge may go light
								parentEntry.paintWedge ();
								}
							}
						}
					if (divSelectedThread !== undefined && divSelectedThread [0] === entry.divThread [0]) {
						divSelectedThread = undefined; //the selected post is going away
						}
					entry.divThread.remove ();
					delete itemsById [item.id];
					if (item.guid !== undefined) {
						delete itemsByGuid [item.guid];
						}
					scheduleRenderMap (); //7/9/26 by CC -- a top-level delete repaints no wedge; the map needs its own nudge
					}
				if (divChat.hasClass ("storyPage") || divChat.hasClass ("profilePage") || divChat.hasClass ("scannerPage")) { //7/5/26 by CC -- #120: only the timeline copy was removed above, so a story, profile or structure page would keep showing the deleted post -- land on the home timeline, the same move as publishing (#136)
					goHomeWithNewPost ();
					}
				};
			this.getCurrentItemInfo = function () { //5/22/26 by Claude -- return chatItem object for the currently selected thread. 6/26 by CC -- read it off the thread so replies and story/profile posts resolve too
				return (getItemForThread (divSelectedThread));
				};
			this.canHoist = function () { //6/27/26 by CC -- #104: the shell enables the Hoist icon when this is true -- there's a selected post and it isn't already the hoisted root. Replies not required; you can hoist a leaf
				const itemRec = getItemForThread (divSelectedThread);
				if (itemRec === undefined) {
					return (false);
					}
				if ((hoistStack.length > 0) && (hoistStack [hoistStack.length - 1] === itemRec.id)) { //already the hoisted root -- can't hoist the same item twice
					return (false);
					}
				return (true);
				};
			this.isHoisted = function () { //6/27/26 by CC -- #104: the shell enables the Dehoist icon when this is true -- we're hoisted into something
				return (hoistStack.length > 0);
				};
			this.hoist = function () { //6/27/26 by CC -- #104: make the cursor post the temporary root. Pushes onto the stack so you can hoist inside a hoist; dehoist walks back up
				const itemRec = getItemForThread (divSelectedThread);
				if (itemRec === undefined) {
					return;
					}
				hoistScrollStack.push (window.scrollY); //remember where the view we're leaving was scrolled, to restore on dehoist
				hoistStack.push (itemRec.id);
				renderHoistedView (itemRec.id);
				logHoistStack ("hoist");
				};
			this.dehoist = function () { //6/27/26 by CC -- #104: undo the last hoist -- pop one level. Back to the previous root, or all the way out to the timeline (cursor on the post you hoisted from) when the stack empties
				if (hoistStack.length === 0) {
					return;
					}
				const poppedId = hoistStack.pop ();
				const restoreScroll = hoistScrollStack.pop (); //the scroll of the view we're going back to
				if (hoistStack.length === 0) {
					returnToTimelineAtItem (poppedId);
					}
				else {
					renderHoistedView (hoistStack [hoistStack.length - 1]);
					}
				window.scrollTo (0, restoreScroll); //restore where that view was scrolled, after the render set its own cursor scroll
				logHoistStack ("dehoist");
				};
			this.getThemeInfo = function () { //5/23/26 by Claude
				return {
					name: "classic",
					version: themesVersion
					};
				};
			this.editNewComment = function (item) { //5/23/26 by Claude
				if (item !== undefined) {
					openReplyModal (item);
					return;
					}
				const theItem = getItemForThread (divSelectedThread); //6/26 by CC -- resolve off the selected thread, covers replies and story/profile posts
				if (theItem !== undefined) {
					openReplyModal (theItem);
					}
				};
			this.editItem = function (item) { //5/23/26 by Claude
				var theItem = item;
				if (theItem === undefined) {
					theItem = getItemForThread (divSelectedThread); //6/26 by CC -- resolve off the selected thread, covers replies and story/profile posts
					}
				if (theItem === undefined) {
					return;
					}
				const flMine = globals.myRssNetwork.userIsSignedIn () && theItem.feedUrl === globals.myRssNetwork.getFeedUrl ();
				if (flMine) {
					openEditModal (theItem);
					}
				};
			this.updateForLogin = updateForLogin;
			this.applyPrefs = applyPrefs;
			this.openEditWindow = openEditWindow;
			this.editNewItem = editNewItem;
			this.toggleLike = function (theItem) { //6/25/26 by CC -- #75: the shell's like-icon click calls this; flip the heart now, tell the server, revert on error. 7/4/26 by CC -- #135: works in every view now -- repaint the timeline copy AND the copy the user clicked (story, profile, hoist), and remember the state on the item so a repeat click computes right
				if (theItem === undefined || theItem.id === undefined) {
					return;
					}
				if (globals.myRssNetwork.userIsSignedIn () === false) {
					speakerBeep (); //7/4/26 by CC -- #130: a signed-out click on the heart beeps instead of doing nothing silently
					return;
					}

				const targets = []; //every rendered heart for this post, in every view -- 7/8/26 by CC: the -1 bug came from copies that missed updates doing their own arithmetic; now all copies move together
				const entry = itemsById [theItem.id]; //the timeline copy, when the post is in the timeline
				if (entry !== undefined) {
					targets.push (entry);
					}
				eachRenderedHeart (theItem.id, function (likeUI) {
					const flSameCopy = (entry !== undefined) && (entry.divLikeIcon !== undefined) && (likeUI.divLikeIcon !== undefined) && (entry.divLikeIcon [0] === likeUI.divLikeIcon [0]);
					if (flSameCopy === false) {
						targets.push (likeUI);
						}
					});

				var flWasLiked = (theItem.flLiked === true);
				if (targets.length > 0 && targets [0].divLikeIcon !== undefined) { //a painted heart is the truth when we have one
					flWasLiked = targets [0].divLikeIcon.hasClass ("liked");
					}
				const flNowLiked = (flWasLiked === false);
				const countDelta = (flNowLiked === true) ? 1 : -1;

				theItem.flLiked = flNowLiked;
				targets.forEach (function (target) {
					paintLikeIcon (target.divLikeIcon, flNowLiked);
					setLikeCountDisplay (target, target.likeCount + countDelta); //7/8/26 by CC -- clamped at zero
					});

				globals.myRssNetwork.toggleLike (theItem.id, function (err, data) {
					if (err) {
						theItem.flLiked = flWasLiked;
						targets.forEach (function (target) {
							paintLikeIcon (target.divLikeIcon, flWasLiked);
							setLikeCountDisplay (target, target.likeCount - countDelta); //7/8/26 by CC -- clamped at zero
							});
						alertDialog (err.message);
						}
					});
				};
			this.showEditor = showEditor; //6/25/26 by CC -- #87: the Show editor menu command reopens the last-edited text
			this.isEditorOpen = function () { //6/25/26 by CC -- #87: true when the edit window is open
				return (divReplyOverlay.is (":visible"));
				};
			this.editTitle = setTitleDialog; //6/10/26 by Claude -- the app's set-title menu command
			this.getEditorData = getEditorData; //6/10/26 by Claude -- the app's view-data menu command

			this.viewStory = function (guid) { //6/20/26 by Claude -- show a post as a story in place; remember where the timeline was scrolled so Back can restore it
				savedTimelineScroll = window.scrollY;
				loadStory (guid);
				};
			this.viewProfile = function (name) { //6/26/26 by CC -- #79: show a user's profile in place; remember the timeline scroll so Back can restore it
				savedTimelineScroll = window.scrollY;
				loadProfile (name);
				};
			this.viewTimeline = function () { //6/20/26 by Claude -- return from a story or profile to the timeline, exactly where it was left
				if (!flTimelineLoaded) { //6/29/26 by CC -- #118: cold-loaded straight into a story -- the timeline behind it was never filled; fill it now
					loadRecentItems ();
					}
				divChat.removeClass ("storyPage");
				divChat.removeClass ("profilePage");
				divChat.removeClass ("scannerPage"); //6/27 by CC -- #104
				divStory.hide ();
				divStory.empty ();
				divProfile.hide ();
				divProfile.empty ();
				divScanner.hide ();
				divScanner.empty ();
				divTimeline.show ();
				window.scrollTo (0, savedTimelineScroll);
				scheduleRenderMap (); //7/9/26 by CC -- back on the timeline; nothing re-renders, so nudge the map by hand
				};
			function viewHome () { //6/29/26 by CC -- #99/#118: Home is all the way back to the top of the main timeline, not a Back button -- clear any hoist, drop the story/profile/scanner overlays, show the timeline, go to the top. 7/4/26 by CC -- #136: now also called internally after publishing, so it's a named function exposed below. 7/11/26 by CC -- the Home icon now comes in through goHome below; this stays the reset-to-top that publishing uses
				if (!flTimelineLoaded) { //6/29/26 by CC -- #118: cold-loaded straight into a story -- the timeline was never filled; fill it now
					loadRecentItems ();
					}
				hoistStack.length = 0;
				hoistScrollStack.length = 0;
				divTimeline.children (".divThread").each (function () { //7/10/26 by CC -- Home puts every open wedge away too; closing a parent restores anything a deeper wedge folded, so top level is enough
					const wedgeUI = $(this).data ("wedgeUI");
					if (wedgeUI !== undefined) {
						wedgeUI.closeReplies ();
						}
					});
				divChat.removeClass ("storyPage");
				divChat.removeClass ("profilePage");
				divChat.removeClass ("scannerPage");
				divStory.hide ();
				divStory.empty ();
				divProfile.hide ();
				divProfile.empty ();
				divScanner.hide ();
				divScanner.empty ();
				divTimeline.show ();
				window.scrollTo (0, 0);
				const divFirstThread = divTimeline.children (".divThread").first (); //7/11/26 by CC -- Home means the top: the cursor lands on the topmost post, not wherever it was before the trip home
				if (divFirstThread.length > 0) {
					selectThread (divFirstThread);
					}
				scheduleRenderMap (); //7/9/26 by CC -- same nudge as viewTimeline
				}
			function canSurface () { //7/10/26 by CC -- Surface: enabled anywhere but the flat timeline -- a story, profile, or flipped/hoisted view has an up to come back from
				return (divChat.hasClass ("storyPage") || divChat.hasClass ("profilePage") || divChat.hasClass ("scannerPage"));
				}
			function surface () { //7/10/26 by CC -- come up from a story, profile, or flipped/hoisted view to the flat timeline, cursor on the post you were just reading, scrolled into view. Distinct from Home, which goes to the top and resets everything
				if (canSurface () === false) {
					return;
					}
				const theItem = getItemForThread (divSelectedThread); //capture before the overlays empty
				if (!flTimelineLoaded) { //cold-loaded straight into a story -- the timeline behind it was never filled; fill it now
					loadRecentItems ();
					}
				hoistStack.length = 0;
				hoistScrollStack.length = 0;
				divChat.removeClass ("storyPage");
				divChat.removeClass ("profilePage");
				divChat.removeClass ("scannerPage");
				divStory.hide ();
				divStory.empty ();
				divProfile.hide ();
				divProfile.empty ();
				divScanner.hide ();
				divScanner.empty ();
				divTimeline.show ();
				scheduleRenderMap (); //back on the timeline; nothing re-renders, so nudge the map by hand

				var entry;
				if (theItem !== undefined) {
					entry = itemsById [theItem.id];
					}
				if (entry !== undefined) {
					selectThread (entry.divThread, true);
					}
				else { //the post isn't in the loaded timeline -- land where the timeline was left
					window.scrollTo (0, savedTimelineScroll);
					}
				}
			this.surface = surface;
			this.canSurface = canSurface;

			function canHome () { //7/10/26 by CC -- the shell enables the Home icon when clicking it would do something: false only on the flat timeline, scrolled to the top, with no replies open under any post
				return (canSurface () || (window.scrollY > 0) || (divTimeline.find (".divReplies .divThread").length > 0));
				}
			this.canHome = canHome;

			function goHome () { //7/11/26 by CC -- Home is two-stage now, the Surface lifeline folded in (prior art: the Home tab in the Twitter and Instagram apps). Buried in a story, profile, flip or open conversation, the first click comes back to the flat timeline with the cursor still on the post you were reading; from the flat timeline it goes to the top and resets, what Home always did
				if (canSurface ()) { //buried in a story, profile or flipped view -- come up, keep your place
					surface ();
					}
				else {
					if (divTimeline.find (".divReplies .divThread").length > 0) { //open conversations in the timeline -- put the nests away, keep your place
						const theItem = getItemForThread (divSelectedThread); //capture before the nests empty
						divTimeline.children (".divThread").each (function () {
							const wedgeUI = $(this).data ("wedgeUI");
							if (wedgeUI !== undefined) {
								wedgeUI.closeReplies ();
								}
							});
						var entry;
						if (theItem !== undefined) {
							entry = itemsById [theItem.id];
							}
						if (entry !== undefined) {
							selectThread (entry.divThread, true); //the post's own timeline row, back on screen if a nest had folded it
							}
						else {
							if (divSelectedThread !== undefined) {
								selectThread (divSelectedThread, true); //closeReplies moved the cursor up to the parent post -- scroll it into view
								}
							}
						}
					else { //the flat timeline -- go to the top and reset
						viewHome ();
						}
					}
				}
			this.viewHome = goHome; //the Home icon's click arrives through the shell's viewHome call; the two-stage behavior lives behind the same name

			function getStructureRoots () { //7/4/26 by CC -- #140: the posts that sit at top level in the structure view -- true top-level posts, plus replies whose parent isn't among the loaded posts (nowhere to nest, so they stand alone honestly)
				const theRoots = new Array ();
				Object.keys (itemsById).forEach (function (id) { //ascending id order is chronological
					const entry = itemsById [id];
					const idParent = entry.item.inReplyTo;
					if (idParent === undefined || itemsById [idParent] === undefined) {
						theRoots.push (entry.item);
						}
					});
				theRoots.reverse (); //newest first, matching the timeline
				return (theRoots);
				}
			function isFlipped () { //7/4/26 by CC -- #140: true when the structure view is showing
				return (divScanner.is (":visible"));
				}
			function canFlip () { //7/4/26 by CC -- #140: the flip belongs to the timeline and the structure view; story and profile pages have their own shapes
				return (!divChat.hasClass ("storyPage") && !divChat.hasClass ("profilePage"));
				}
			function flipView () { //7/4/26 by CC -- #140: flip between the flat timeline and the structure view -- the same posts, nested under their parents. The cursor's post stays highlighted and holds its height on screen while the view reorganizes around it
				if (canFlip () === false) {
					return;
					}
				const selectedItem = getItemForThread (divSelectedThread);
				var savedOffset;
				if (divSelectedThread !== undefined) {
					savedOffset = divSelectedThread [0].getBoundingClientRect ().top;
					}
				function restoreCursor (theContainer) { //find the cursor's post in the freshly shown view, select it, put it back at the same height
					var divNewThread;
					if (selectedItem !== undefined) {
						theContainer.find (".divThread").each (function () {
							const threadItem = $(this).data ("item");
							if (divNewThread === undefined && threadItem !== undefined && threadItem.id === selectedItem.id) {
								divNewThread = $(this);
								}
							});
						}
					if (divNewThread === undefined) {
						window.scrollTo (0, 0);
						}
					else {
						selectThread (divNewThread);
						const newOffset = divNewThread [0].getBoundingClientRect ().top;
						window.scrollBy (0, newOffset - savedOffset);
						}
					}
				if (isFlipped ()) { //back to the flat timeline
					divChat.removeClass ("scannerPage");
					divScanner.hide ();
					divScanner.empty ();
					divTimeline.show ();
					scheduleRenderMap (); //7/9/26 by CC -- back on the timeline; nothing re-renders, so nudge the map by hand
					restoreCursor (divTimeline);
					}
				else { //to the structure view
					divScanner.empty ();
					getStructureRoots ().forEach (function (rootItem) {
						renderSubtree (rootItem, divScanner);
						});
					openScannerArea ();
					restoreCursor (divScanner);
					}
				}
			this.flipView = flipView;
			this.isFlipped = isFlipped;
			this.canFlip = canFlip;
			}