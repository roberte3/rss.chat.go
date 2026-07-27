//commands -- 6/5/26 by DW
	function createAccountCommand () {
		console.log ("createAccountCommand");
		function canCreateAccountCallback (email, name, callback) { //6/18/26 by DW
			if (globals.userData.flWhitelist) {
				checkWithWhitelist (email, function (err, flWhitelisted) {
					if (err) {
						const message = "Can't create an account for \"" + email + "\" because there was an error authorizing the account.";
						callback ({message});
						}
					else {
						if (flWhitelisted) {
							callback (undefined);
							}
						else {
							const message = "Can't create the account because \"" + email + "\" is not authorized to use this service.";
							callback ({message});
							}
						}
					})
				}
			else {
				callback (undefined);
				}
			}
		const signupOptions = {
			extraLegalChars: ["."],
			canCreateAccountCallback
			};
		signupDialog (signupOptions);
		}
	function signInCommand () {
		console.log ("signInCommand");
		const defaultAddress = (localStorage.lastEmailAccount !== undefined) ? localStorage.lastEmailAccount : "";
		askDialog ("Enter your email address:", defaultAddress, "you@example.com", function (email, flCancelled) {
			if (!flCancelled) {
				email = trimWhitespace (email);
				if (!legalEmailAddress (email)) {
					alertDialog ("Can't sign in because \"" + email + "\" is not a legal email address.");
					}
				else {
					checkWithWhitelist (email, function (err, flWhitelisted) {
						if (err) {
							alertDialog ("Can't open an account for \"" + email + "\" because there was an error authorizing the account.");
							}
						else {
							if (flWhitelisted) {
								localStorage.lastEmailAccount = email; 
								globals.myRssNetwork.signIn (email, function (err, data) {
									if (err) {
										alertDialog (err.message);
										}
									else {
										alertDialog (data.message); //Please check your email, probably.
										}
									});
								}
							else {
								alertDialog ("Can't open the account because \"" + email + "\" is not authorized to use this service.");
								}
							}
						})
					}
				}
			});
		}
	function signOutCommand () {
		confirmDialog ("OK to sign out?", function () { //6/5/26 by DW -- added confirmation
			globals.myRssNetwork.signOut ();
			updateForLogin ();
			});
		}
	function settingsCommand () { //5/16/26 by DW
		console.log ("settingsCommand"); 
		const flReturnCloses = false; //5/28/26 by DW -- we have a textarea, without this no way to enter a newline
		prefsDialogShow (function () {
			savePrefsAndReload ();
			}, flReturnCloses);
		}
	function viewMyFeedCommand () { //6/5/26 by DW
		console.log ("viewMyFeedCommand");
		if (isLoggedIn ()) {
			const feedUrl = globals.myRssNetwork.getFeedUrl ();
			window.open (feedUrl);
			}
		else {
			speakerBeep ();
			}
		}
	function viewEveryonesFeedCommand () { //6/5/26 by DW
		console.log ("viewEveryonesFeedCommand");
		window.open (globals.userData.feedUrlEveryone);
		}
	function viewUserlistCommand () { //6/5/26 by DW
		console.log ("viewUserlistCommand");
		window.open (globals.userData.opmlListUrl);
		}
	function supportCommand () { //6/5/26 by DW
		alertDialog ("TBD");
		}
	function deletePostCommand (theItem) { //6/12/26 by DW
		confirmDialog ("OK to delete the selected post? (There is no undo.)", function () {
			globals.myRssNetwork.deletePost (theItem.id, function (err, data) { 
				if (err) {
					alertDialog (err.message);
					}
				else {
					console.log ("deleteItemCommand: data == " + jsonStringify (data));
					globals.myChatUserInterface.removeItem (theItem);
					speakerBeep ();
					}
				});
			});
		}
	
	function clickEyeIcon (item) { //6/25/26 by CC
		const url = location.origin + "?id=" + item.id;
		history.pushState ({id: item.id}, "", url);
		globals.myChatUserInterface.viewStory (url);
		}
	function testEyeIcon () { //6/19/26 by DW
		const theGuid = "https://rss.network/permalink?screenname=dave.winer&id=s0uegzjn";
		const url = location.origin + "?storyguid=" + encodeURIComponent (theGuid);
		window.open (url);
		}
	
	function clickHomeButton () { //6/19/26 by DW
		history.pushState ({}, "", location.origin);
		globals.myChatUserInterface.viewHome ();
		}
	function newPostCommand () { //6/25/26 by DW
		globals.myChatUserInterface.editNewItem (); 
		}
	function showEditorCommand () { //6/25/26 by DW
		if (globals.myChatUserInterface.isEditorOpen ()) {
			speakerBeep ();
			}
		else {
			globals.myChatUserInterface.showEditor (); 
			}
		}
	
	
	
	function versionsDialog () { //6/28/26 by DW
		console.log ("versionsDialog");
		
		const serverAddress = settingsFromServer.urlServer;
		const linkToServer = "<a href=\"" + serverAddress + "\" target=\"_blank\">" + serverAddress + "</a>";
		const productname = settingsFromServer.productNameForDisplay;
		
		var htmltext = "";
		function add (s) {
			htmltext += s;
			}
		
		add ("<div class=\"divAboutDialogText\">");
		add ("<b>Software versions</b>");
		add ("<ul>");
		add ("<li>Server address: " + settingsFromServer.urlServer + "</li>");
		add ("<li>Server version: v" + globals.userData.serverVersion + ".</li>"); //7/1/26 by DW
		add ("<li>Client version: v" + appConsts.version + ".</li>"); //7/2/26 by DW
		add ("<li>Current theme: " + firstCharUpper (appPrefs.currentThemeName) + " v" + getThemeVersion () + "</li>");
		
		var serverName = "MySQL"; //7/21/26 by DW
		if (globals.userData.databaseEngine !== undefined) {
			serverName = globals.userData.databaseEngine;
			}
		add ("<li>" + serverName + " version: v" + globals.userData.mySqlVersion + "</li>");
		
		add ("</ul>");
		add ("</div>");
		
		alertDialog (htmltext);
		}
	
	
	
//handling socket items -- 4/23/26 by DW
	var idsSeen = new Array ();
	
	function ageoutIdsSeen () { //age out items in idsSeen array
		var newArray = new Array ();
		idsSeen.forEach (function (item) {
			if (secondsSince (item.when) < appConsts.maxSecsForIds) {
				newArray.push (item);
				}
			else {
				}
			});
		idsSeen = newArray;
		}
	
	function isFeedInExtras (feedUrl) { //5/18/26 by DW
		
		return (false); //feature disabled -- 5/18/26 by DW
		
		var flFound = false;
		globals.userData.extrasList.forEach (function (xmlUrl) {
			if (xmlUrl === feedUrl) {
				flFound = true;
				}
			});
		return (flFound);
		}
	function includeSocketItem (theItem) { 
		//exclude items we've already seen
			var flInclude = true; 
			idsSeen.forEach (function (item) {
				if (item.id === theItem.id) {
					flInclude = false;
					}
				});
			if (flInclude) {
				idsSeen.push ({
					id: theItem.id,
					when: new Date ()
					})
				}
			else {
				return (false); //don't include, already seen
				}
		//include items from our feeds
			if (beginsWith (theItem.feedUrl, globals.userData.baseFeedUrl)) {
				if (theItem.feedUrl == globals.userData.feedUrlEveryone) { //6/8/26 by DW
					return (false);
					}
				else {
					return (true);
					}
				}
			else {
				if (isFeedInExtras (theItem.feedUrl)) {
					return (true);
					}
				else {
					return (appConsts.flIncludeAllSocketItems);
					}
				}
		}
	
	function socketItemHandler (flNew, theFeed, theItem) { //6/21/26 by CC
		if (includeSocketItem (theItem)) {
			console.log ("socketItemHandler: flNew == " + flNew + ", feedUrl == " + theItem.feedUrl);
			if (flNew) {
				globals.myChatUserInterface.newItem (theItem);
				}
			else {
				globals.myChatUserInterface.updateItem (theItem);
				}
			}
		}
	
	function socketNewItemCallback (theFeed, theItem) { 
		socketItemHandler (true, theFeed, theItem); 
		}
	function socketUpdatedItemCallback (theFeed, theItem) {
		socketItemHandler (false, theFeed, theItem); 
		}
//themes -- 5/1/26 by DW
	function updateTheme (themeName, callback) {
		const cacheConfuser = "?x=" + new Date ().getTime (), whenstart = new Date ();
		if (themeName === undefined) {
			themeName = appPrefs.currentThemeName;
			}
		console.log ("switchTheme: themeName == " + themeName);
		const baseUrl = appConsts.urlThemes + themeName + "/";
		
		function installJavaScriptCode (theCode) {
			$("#scriptTheme").remove ();
			const scriptTheme = document.createElement ("script");
			scriptTheme.id = "scriptTheme";
			scriptTheme.textContent = theCode;
			document.head.appendChild (scriptTheme);
			}
		function installCssStyles (theStyles) {
			$("#styleTheme").remove ();
			const styleTheme = document.createElement ("style");
			styleTheme.id = "styleTheme";
			styleTheme.textContent = theStyles;
			document.head.appendChild (styleTheme);
			}
		
		const urlStyles = baseUrl + "theme.css" + cacheConfuser;
		httpRequest (urlStyles, undefined, undefined, function (err, cssText) {
			if (err) {
				console.log ("updateTheme: urlStyles == " + urlStyles + ", err.message == " + err.message);
				const message = "Can't boot up, because we failed to load theme styles from the server.";
				callback ({message});
				}
			else {
				const urlCode = baseUrl + "theme.js" + cacheConfuser;
				httpRequest (urlCode, undefined, undefined, function (err, jsText) {
					if (err) {
						console.log ("updateTheme: urlCode == " + urlCode + ", err.message == " + err.message);
						const message = "Can't boot up, because we failed to load theme code from the server.";
						callback ({message});
						}
					else {
						installJavaScriptCode (jsText)
						installCssStyles (cssText);
						console.log ("updateTheme: " + secondsSince (whenstart) + " secs.");
						callback (undefined);
						}
					});
				}
			});
		}
	function itemSelectedCallback (theItem) { //6/1/26 by DW -- called when user clicks on a timeline item
		globals.currentTimelineItem = theItem;
		enableMenuItems (); //6/10/26 by DW
		enableIcons (); //6/12/26 by DW
		setLastMessageRead (theItem); //6/27/26 by DW
		}
	function enableIcons () { //6/1/26 by DW
		function enableItemIcons () {
			insideItemIcons.forEach (function (item) {
				switch (item.name) {
					case "comment":
						item.enabled = true;
						break
					case "pencil":
						item.enabled = userCanEditTimelineItem (); //6/9/26 by DW
						break
					case "eye":
						item.enabled = enableEyeIcon (); //6/9/26 by DW
						break
					case "trashcan":
						item.enabled = userCanEditTimelineItem ();
						break
					}
				});
			}
		function enableGlobalIcons () {
			leftColumnIcons.forEach (function (item) {
				switch (item.name) {
					case "home":
						item.enabled = globals.myChatUserInterface.canHome ();
						break
					case "bigPlus":
						item.enabled = isLoggedIn ();
						break
					case "feed":
						item.enabled = isLoggedIn ();
						break
					case "data":
						item.enabled = isLoggedIn ();
						break
					case "flipswitch": //7/4/26 by DW
						item.enabled = globals.myChatUserInterface.canFlip ();
						break;
					case "surface": //7/10/26 by DW
						item.enabled = globals.myChatUserInterface.canSurface ();
						break;
					}
				});
			}
		enableItemIcons ();
		enableGlobalIcons ();
		}
	function enableMenuItems () { //6/10/26 by DW
		timelineItemMenu.forEach (function (item) {
			switch (item.name) {
				case "edit":
					item.enabled = userCanEditTimelineItem (); 
					break
				case "delete": //7/5/26 by CC
					item.enabled = userCanEditTimelineItem (); 
					break
				}
			});
		}
	
	function avatarClickedCallback (screenname, ev) { //6/15/26 by DW
		const url = location.origin + "?screenname=" + screenname;
		history.pushState ({name: screenname}, "", url);
		globals.myChatUserInterface.viewProfile (screenname);
		}
	
	
//socket --7/17/26 by DW
	function firehoseSocket (userOptions) {
		console.log ("firehoseSocket");
		
		const options = {
			flWebsocketEnabled: true,
			urlFirehoseSocket: undefined,
			
			maxRetries: 100, //when we lose a connection, we try to reconnect this many times
			ctSecsBetwRetries: 10,
			initialCheckTimeout: 100,
			
			flLogNewItem: false,
			flLogUpdatedItem: false,
			
			newItemCallback: function (theFeed, theItem) {
				},
			updatedItemCallback: function (theFeed, theItem) {
				},
			
			getSocketGreeting: function () { //7/16/26 by CC -- returns "user email code" when signed in, undefined when not
				return (undefined);
				},
			goodnightDialogMsg: "The app is running in another tab or browser. Click OK to reload this one, or you can safely close it.", //7/16/26 by CC
			
			maxSecsBetwNotifications: 10.1, //7/4/26 by DW -- a notice on the same id less than x secs apart are considered to be the same one
			};
		mergeOptions (userOptions, options);
		
		var ctNewItems = 0, ctUpdatedItems = 0;
		var recentIds = new Object ();
		function notSeenRecently (id) {
			var flSeen = false;
			function ageOut () {
				var newObject = new Object ();
				for (var x in recentIds) {
					if (secondsSince (recentIds [x]) <= options.maxSecsBetwNotifications) {
						newObject [x] = recentIds [x];
						}
					}
				recentIds = newObject;
				}
			ageOut (); //remove expired ids
			for (var x in recentIds) {
				if (id == x) {
					flSeen = true;
					}
				}
			recentIds [id] = new Date ();
			return (!flSeen);
			}
		
		function handleMessage (theCommand, thePayload) {
			function getTitle (item) {
				if (item.title === undefined) {
					return (maxStringLength (stripMarkup (item.description), 35));
					}
				else {
					return (item.title);
					}
				}
			
			if (thePayload.item !== undefined) { //debugging
				switch (theCommand) {
					case "newItem":
						ctNewItems++;
						var wpData = "";
						if (thePayload.item.metadata !== undefined) {
							if (thePayload.item.metadata.wpSiteId !== undefined) {
								wpData = thePayload.item.metadata.wpSiteId + "/" + thePayload.item.metadata.wpPostId;
								}
							}
						if (options.flLogNewItem) {
							console.log (`${nowstring ()} ${theCommand} ${thePayload.item.feedUrl} ${wpData}`);
							}
						options.newItemCallback (thePayload.theFeed, thePayload.item)
						break;
					case "updatedItem": //11/16/25 by DW
						ctUpdatedItems++;
						if (options.flLogUpdatedItem) {
							console.log (`${nowstring ()} ${theCommand} ${thePayload.item.feedUrl}`);
							}
						options.updatedItemCallback (thePayload.theFeed, thePayload.item);
						break;
					}
				}
			}
		
		var mySocket = undefined, idSocketChecker;
		var ctRetries = 0;
		var flGoodnightDialogShowing = false;
		
		function handleGoodnightMessage () { //7/16/26 by CC
			if (!flGoodnightDialogShowing) {
				flGoodnightDialogShowing = true;
				mySocket.close (1000, "Received goodnight message."); //1000 is the code for normal closure
				const theDialog = alertDialog (options.goodnightDialogMsg, function () {
					location.reload (true);
					});
				theDialog.on ("hidden", function () { //7/19/26 by CC
					//reload however the dialog is dismissed, incl clicking outside it
					location.reload (true);
					});
				}
			}
		function checkConnection () {
			if ((mySocket === undefined) && (!flGoodnightDialogShowing)) { //don't reopen the socket after being told to go away
				mySocket = new WebSocket (options.urlFirehoseSocket);
				mySocket.onopen = function (evt) {
					ctRetries = 0; //we got through
					const greeting = options.getSocketGreeting ();
					if (greeting !== undefined) {
						mySocket.send (greeting);
						}
					};
				mySocket.onmessage = function (evt) {
					function getPayload (jsontext) {
						var thePayload = undefined;
						try {
							thePayload = JSON.parse (jsontext);
							}
						catch (err) {
							}
						return (thePayload);
						}
					if (evt.data !== undefined) { //no error
						if (evt.data == "goodnight") { //7/16/26 by CC -- no payload on this one, handle it before the parsing below
							handleGoodnightMessage ();
							return;
							}
						var theCommand = stringNthField (evt.data, "\r", 1);
						var jsontext = stringDelete (evt.data, 1, theCommand.length + 1);
						var thePayload = getPayload (jsontext);
						handleMessage (theCommand, thePayload);
						}
					};
				mySocket.onclose = function (evt) {
					mySocket = undefined;
					if (ctRetries++ >= options.maxRetries) {
						clearInterval (idSocketChecker);
						}
					};
				mySocket.onerror = function (evt) {
					console.log ("firehoseSocket: socket received an error.");
					};
				}
			}
		
		if (options.flWebsocketEnabled) {
			setTimeout (function () {
				checkConnection ();
				idSocketChecker = setInterval (checkConnection, 1000 * options.ctSecsBetwRetries);
				}, options.initialCheckTimeout);
			}
		}
//startup support -- 7/13/26 by DW
	function simpleInits () { //7/13/26 by DW -- startup things that don't need to be waited for
		startTurndown (); //7/22/26 by DW
		$(".divMenuProductName").text (settingsFromServer.productNameForDisplay); //7/13/26 by DW
		if (endsWith (getDomainFromUrl (settingsFromServer.urlServer), "rss.chat")) { //7/25/26 by DW
			hitCounter (); 
			}
		}
	function everySecond () {
		if (flPrefsChanged) { //5/16/26 by DW
			flPrefsChanged = false;
			savePrefs ();
			}
		enableIcons (); //6/1/26 by DW
		enableMenuItems (); //6/10/26 by DW
		}
	function everyMinute () {
		ageoutIdsSeen () ;
		$(".spUpdateableTime").trigger ("update"); //5/6/26 by DW
		}
	
	function startPackages (callback) {
		
		function completeStartPackages () { //5/22/26 by DW
			
			var storyGuid = undefined; //6/19/26 by DW
			var params = getAllUrlParams ();
			console.log ("startPackages: params == " + jsonStringify (params));
			if (params.id !== undefined) {
				storyGuid = getPermalinkUrl (params.id);
				}
			
			const chatOptions = {
				whereToAppend: $(".divChatContainer"),
				iconBar: leftColumnIcons, //5/20/26 by DW
				itemIconBar: insideItemIcons, //5/22/26 by DW
				placeholderSignedIn: appPrefs.placeholderSignedIn, //5/24/26 by DW
				placeholderSignedOut: appPrefs.placeholderSignedOut,
				placeholderReply: appPrefs.placeholderReply,
				itemSelectedCallback, //6/1/26 by DW
				titleTooltip: "Click to add an optional title.", //6/7/26 by DW
				timelineItemMenu, //6/10/26 by DW
				editorMenu, //6/10/26 by DW
				avatarClickedCallback, //6/15/26 by DW
				storyGuid, //6/19/26 by DW
				profileName: params.screenname, //6/26/26 by DW
				idLastMessageRead: appPrefs.idLastMessageRead, //6/27/26 by DW
				itemSeenCallback: setLastMessageRead, //7/2/26 by DW -- #129: a post that's been on screen is read
				};
			globals.myChatUserInterface = new chatUserInterface (chatOptions);
			globals.myChatUserInterface.applyPrefs (); //5/19/26 by DW
			
			const socketOptions = {
				urlFirehoseSocket:  appConsts.urlSocketServer, //6/24/26 by DW
				flWebsocketEnabled: appConsts.flSocketsEnabled, //7/20/26 by CC -- #186
				newItemCallback: socketNewItemCallback, //4/20/26 by DW
				updatedItemCallback: socketUpdatedItemCallback,
				getSocketGreeting: function () { //7/17/26 by CC
					return (globals.myRssNetwork.getSocketGreeting ());
					},
				goodnightDialogMsg: getGoodbyDialogMessage (), //7/17/26 by CC
				}
			globals.myFirehoseSocket = new firehoseSocket (socketOptions); 
			
			window.addEventListener ("popstate", function (ev) {
				const params = getAllUrlParams ();
				switch (true) {
					case (params.screenname !== undefined): 
						globals.myChatUserInterface.viewProfile (params.screenname);
						break;
					case (params.id !== undefined): 
						globals.myChatUserInterface.viewStory (getPermalinkUrl (params.id));
						break;
					default: 
						globals.myChatUserInterface.viewTimeline ();
						break
					}
				});
			
			callback ();
			}
		
		const rssNetOptions = {
			serverAddress: appConsts.urlServer,
			};
		globals.myRssNetwork = new rssNetworkServer (rssNetOptions);
		
		if (!globals.myRssNetwork.willRedirect ()) {
			globals.myRssNetwork.start (function (err, userData) { 
				if (err) { //6/13/26 by DW
					alertDialog (err.message);
					}
				else {
					if (userData.prefs !== undefined) { //5/16/26 by DW
						if (userData.prefs.userCss !== undefined) { //5/28/26 by DW -- gives the user an easy way to start over
							if (trimWhitespace (userData.prefs.userCss).length == 0) {
								userData.prefs.userCss = appPrefs.userCss;
								}
							}
						for (var x in userData.prefs) {
							appPrefs [x] = userData.prefs [x];
							}
						}
					globals.userData = userData;
					appPrefs.ctStarts++;
					appPrefs.whenLastStart = new Date ();
					if (appPrefs.lastEmail === undefined) { //5/27/26 by DW
						appPrefs.lastEmail = getEmailAddress ();
						}
					fixPrefs (); //7/2/26 by DW
					prefsChanged ();
					updateTheme (undefined, function (err) { //5/30/26 by DW
						if (err) {
							alertDialog (err.message);
							}
						else {
							if (appPrefs.flApplyUserCss) { //5/28/26 by DW
								applyUserCss (appPrefs.userCss);
								}
							completeStartPackages (); //5/22/26 by DW
							}
						});
					}
				});
			}
		}
function startup () {
	console.log ("startup -- 6/5/26 by DW");
	simpleInits (); //7/13/26 by DW
	startPackages (function () {
		
		
		updateForLogin ();
		
		startTurndown ();  //5/19/26 by DW
		
		runEveryMinute (everyMinute);
		self.setInterval (everySecond, 1000);
		
		});
	}
