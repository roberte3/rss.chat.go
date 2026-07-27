function chatUserInterface (userOptions) { //4/29/26 by Claude + DW
	console.log ("chatUserInterface");
	
	var options = {
		whereToAppend: undefined, //i want the designer to tell us where it goes -- 4/29/26 by DW
		micIcon: "&#x1F3A4;", //microphone
		sendIcon: "&#x27A4;", //rightward arrow
		placeholderSignedIn: "Message",
		placeholderSignedOut: "Sign in to post",
		ctRecentItems: 100,
		};
	mergeOptions (userOptions, options);
	
	const divChat = $("<div class=\"divChat\"></div>");
	
	var getEditorText, clearEditor, setEditorEnabled; //assigned by getTextEditor
	var onEditorSubmit, onEditorInput; //callbacks assigned by chat constructor
	
	const sendButton = $("<button class=\"sendButton\">" + options.micIcon + "</button>");
	const divMessageContainer = $("<div class=\"divMessageContainer\"></div>");
	
	const itemsById = {}; //look up parent bubble when an item with inReplyTo arrives
	
	var theLastFeedUrl = undefined;
	var theLastDateString = undefined;
	
	function getTimeString (theDate) {
		return (theDate.toLocaleTimeString ([], {hour: "2-digit", minute: "2-digit"}));
		}
	function getDateLabel (theDate) {
		const now = new Date ();
		const today = new Date (now.getFullYear (), now.getMonth (), now.getDate ());
		const dateOnly = new Date (theDate.getFullYear (), theDate.getMonth (), theDate.getDate ());
		const diffDays = (today - dateOnly) / 86400000;
		if (diffDays < 1) {
			return ("Today");
			}
		if (diffDays < 2) {
			return ("Yesterday");
			}
		return (theDate.toLocaleDateString ([], {month: "long", day: "numeric", year: "numeric"}));
		}
	
	function addItemToTimeline (item, flScroll) { //adds item to display
		const theDate = new Date (item.pubDate);
		const flMine = globals.myRssNetwork.userIsSignedIn () && (item.feedUrl === globals.myRssNetwork.getFeedUrl ());
		const dateString = theDate.toDateString ();
		
		function getDateDivider () {
			const divDateDivider = $("<div class=\"divDateDivider\"></div>");
			const spanLabel = $("<span></span>").text (getDateLabel (theDate));
			divDateDivider.append (spanLabel);
			return (divDateDivider);
			}
		function getSenderName () {
			const divSenderName = $("<div class=\"divSenderName\"></div>").text (item.author);
			return (divSenderName);
			}
		function getBubbleText () {
			const divBubbleText = $("<div class=\"divBubbleText\"></div>").text (item.description);
			return (divBubbleText);
			}
		function getBubbleMeta () {
			const divBubbleMeta = $("<div class=\"divBubbleMeta\"></div>");
			const spanTimestamp = $("<span class=\"divTimestamp\">" + getTimeString (theDate) + "</span>");
			divBubbleMeta.append (spanTimestamp);
			if (flMine) {
				const spanTicks = $("<span class=\"divTicks\">&#x2713;&#x2713;</span>"); //double checkmark
				divBubbleMeta.append (spanTicks);
				}
			return (divBubbleMeta);
			}
		function getBubble (flTail) {
			const sideClass = flMine ? "mine" : "theirs";
			const tailClass = flTail ? "tail" : "noTail";
			const divBubble = $("<div class=\"divBubble " + sideClass + " " + tailClass + "\"></div>");
			if (!flMine && flTail) {
				divBubble.append (getSenderName ());
				}
			divBubble.append (getBubbleText ());
			divBubble.append (getBubbleMeta ());
			return (divBubble);
			}
		
		const parentEntry = (item.inReplyTo === undefined) ? undefined : itemsById [item.inReplyTo];
		var divParent, flTail;
		if (parentEntry === undefined) { //top-level item -- date divider, tail by feedUrl change, attach to message container
			if (dateString !== theLastDateString) {
				divMessageContainer.append (getDateDivider ());
				theLastDateString = dateString;
				theLastFeedUrl = undefined;
				}
			flTail = (item.feedUrl !== theLastFeedUrl);
			theLastFeedUrl = item.feedUrl;
			divParent = divMessageContainer;
			}
		else { //reply -- always show sender, attach inside parent's divReplies
			flTail = true;
			divParent = parentEntry.divReplies;
			}
		const divBubble = getBubble (flTail);
		const divReplies = $("<div class=\"divReplies\"></div>");
		divBubble.append (divReplies);
		divParent.append (divBubble);
		itemsById [item.id] = {divBubble, divReplies};
		
		if (flScroll) {
			divMessageContainer.scrollTop (divMessageContainer [0].scrollHeight);
			}
		}
	
	function sendMessage () {
		console.log ("sendMessage");
		const text = getEditorText ();
		if (text.length === 0) {
			return;
			}
		if (!globals.myRssNetwork.userIsSignedIn ()) {
			return;
			}
		const item = {
			feedUrl: globals.myRssNetwork.getFeedUrl (),
			author: globals.myRssNetwork.getScreenname (),
			description: text,
			pubDate: new Date ().toISOString ()
			};
		if (appConsts.flPostItemsLocally) { //4/24/26 by DW
			addItemToTimeline (item, true);
			}
		clearEditor ();
		sendButton.html (options.micIcon);
		const postRec = {
			description: text
			};
		globals.myRssNetwork.newPost (postRec, function (err, data) {
			if (err) {
				console.log ("sendItem: err.message == " + err.message);
				}
			});
		}
	
	function refresh () {
		setEditorEnabled (globals.myRssNetwork.userIsSignedIn ());
		}
	function loadRecentItems () { //4/29/26 by DW
		globals.myRssNetwork.getRecentItems (options.ctRecentItems, function (err, theItems) {
			if (err) {
				console.log ("loadRecentItems: err.message == " + err.message);
				}
			else {
				theItems.reverse (); //display oldest first, newest at bottom
				theItems.forEach (function (item) {
					addItemToTimeline (item, false);
					});
				divMessageContainer.scrollTop (divMessageContainer [0].scrollHeight);
				}
			});
		}
	
	function getHeader () {
		const divHeader = $("<div class=\"divHeader\"></div>");
		const divBackArrow = $("<div class=\"divBackArrow\">&#x2190;</div>"); //leftwards arrow
		const divAvatar = $("<div class=\"divAvatar\">r</div>");
		const divHeaderInfo = $("<div class=\"divHeaderInfo\"></div>");
		const divHeaderName = $("<div class=\"divHeaderName\">rss.network</div>");
		const divHeaderStatus = $("<div class=\"divHeaderStatus\">online</div>");
		divHeaderInfo.append (divHeaderName);
		divHeaderInfo.append (divHeaderStatus);
		const divHeaderIcons = $("<div class=\"divHeaderIcons\"><span title=\"More\">&#x22EE;</span></div>"); //vertical ellipsis
		divHeader.append (divBackArrow);
		divHeader.append (divAvatar);
		divHeader.append (divHeaderInfo);
		divHeader.append (divHeaderIcons);
		return (divHeader);
		}
	function getTextEditor () {
		const divTextEditor = $("<div class=\"divTextEditor\"></div>");
		const editorInput = $("<input type=\"text\" class=\"editorInput\" />");
		divTextEditor.append (editorInput);
		
		editorInput.on ("input", function () {
			if (onEditorInput !== undefined) {
				onEditorInput ();
				}
			});
		editorInput.on ("keydown", function (event) {
			if (event.key === "Enter") {
				if (onEditorSubmit !== undefined) {
					onEditorSubmit ();
					}
				}
			});
		
		getEditorText = function () {
			return (editorInput.val ().trim ());
			};
		clearEditor = function () {
			editorInput.val ("");
			};
		setEditorEnabled = function (flBool) {
			if (flBool) {
				editorInput.prop ("disabled", false);
				editorInput.attr ("placeholder", options.placeholderSignedIn);
				}
			else {
				editorInput.prop ("disabled", true);
				editorInput.attr ("placeholder", options.placeholderSignedOut);
				}
			};
		
		return (divTextEditor);
		}
	function getInputBar () {
		const divInputBar = $("<div class=\"divInputBar\"></div>");
		const divEmojiButton = $("<div class=\"divEmojiButton\">&#x1F642;</div>"); //slightly smiling face
		divInputBar.append (divEmojiButton);
		divInputBar.append (getTextEditor ());
		divInputBar.append (sendButton);
		return (divInputBar);
		}
	
	divChat.append (getHeader ());
	divChat.append (divMessageContainer);
	divChat.append (getInputBar ());
	options.whereToAppend.append (divChat);
	
	onEditorInput = function () {
		sendButton.html (getEditorText () ? options.sendIcon : options.micIcon);
		};
	onEditorSubmit = sendMessage;
	sendButton.on ("click", function () {
		if (getEditorText ()) {
			sendMessage ();
			}
		});
	
	refresh ();
	loadRecentItems (); //4/29/26 by DW
	
	this.newItem = function (item) {
		console.log ("chatUserInterface.newItem");
		addItemToTimeline (item, true);
		};
	this.refresh = refresh;
	}
