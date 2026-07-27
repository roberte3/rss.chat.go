function prefsChanged () {
	flPrefsChanged = true;
	}
function savePrefs (callback) {
	const whenstart = new Date (); //6/29/26 by DW
	globals.myRssNetwork.savePrefs (appPrefs, function (err) {
		if (err) {
			console.log ("savePrefs: err.message == " + err.message);
			if (callback !== undefined) { //5/24/26 by DW
				callback (err);
				}
			}
		else {
			console.log (nowstring () + " savePrefs: " + secondsSince (whenstart) + " secs."); //6/29/26 by DW
			if (callback !== undefined) { //5/24/26 by DW
				callback ();
				}
			}
		});
	}
function savePrefsAndReload () { //11/1/24 by DW
	savePrefs (function (err) { 
		if (err) {
			alertDialog (err.message);
			}
		else {
			location.reload ();
			}
		});
	}
function legalEmailAddress (email) { //4/19/26 by DW
	if (email.length == 0) {
		return (false);
		}
	else {
		if (stringContains (email, "@")) {
			var domain = stringNthField (email, "@", 2);
			if (stringContains (domain, ".")) {
				return (true);
				}
			else {
				return (false); //must contain a period
				}
			}
		else {
			return (false);
			}
		}
	}
function getItemText (item) {
	var itemtext = item.title;
	if (itemtext === undefined) {
		itemtext = maxStringLength (stripMarkup (item.description), appConsts.maxCharsItemText, true, appConsts.flEllipsesAfterText);
		}
	itemtext = trimWhitespace (itemtext);
	
	if (appConsts.flEllipsesAfterText) {
		if (itemtext.length > 0) {
			if (!isPunctuation (itemtext [itemtext.length - 1])) {
				itemtext += ".";
				}
			}
		}
	
	return (itemtext);
	}
function formatTime (theDate) {
	theDate = new Date (theDate);
	const theString = theDate.toLocaleTimeString ([], {hour: "2-digit", minute: "2-digit"});
	return (theString);
	}
function getFeedlandTimeString (when, flLongStrings=false) {
	const options = {
		flBriefYearDates: true,
		nowString: "now",
		flUseYesterdayString: false //2/10/24 by DW
		};
	var s = getFacebookTimeString (when, flLongStrings, options);
	return (s);
	}
function getUpdateableTime (when, flTextAtEnd=".", flLongString=true, link) { //8/28/22 by DW
	var theWhen = $("<span class=\"spUpdateableTime\"></span>");
	function setWhen () {
		var whenstring = getFeedlandTimeString (when, flLongString) + flTextAtEnd;
		if (link !== undefined) { //9/1/22 by DW
			whenstring = "<a href=\"" + link + "\" target=\"_blank\">" + whenstring + "</a>";
			}
		if (theWhen.html () != whenstring) { //avoid flashing in the debugger, it's annoying
			theWhen.html (whenstring);
			}
		}
	setWhen ();
	theWhen.on ("update", setWhen);
	return (theWhen);
	}
function isLoggedIn () {
	const flSignedIn = globals.myRssNetwork.userIsSignedIn ();
	return (flSignedIn);
	}
function getMyFeedUrl () {
	if (isLoggedIn ()) {
		const feedUrl = globals.myRssNetwork.getFeedUrl ();
		return (feedUrl);
		}
	return null;
	}
function updateForLogin () { //6/5/26 by DW
	if (isLoggedIn ()) {
		$("#idCreateAccount").hide ();
		$("#idSignIn").hide ();
		$("#idSignOut").show ();
		$("#idViewMyFeed").show ();
		$("#idUsername").text (getScreenname ()); //6/5/26 by DW
		}
	else {
		$("#idCreateAccount").show ();
		$("#idSignIn").show ();
		$("#idSignOut").hide ();
		$("#idViewMyFeed").hide ();
		$("#idUsername").text ("Sign in here."); //6/5/26 by DW
		}
	globals.myChatUserInterface.updateForLogin ();
	}
function startTurndown () { //5/5/25 by DW
	myTurndown = new TurndownService ();
	}
function getMarkdownFromHtml (htmltext) { //4/25/24 by DW
	const markdowntext = myTurndown.turndown (htmltext);
	return (markdowntext);
	}
function getHtmlFromMarkdown (mdtext) { //5/19/26 by DW
	console.log ("getHtmlFromMarkdown");
	const md = new Markdown.Converter ();
	const htmltext = md.makeHtml (mdtext);
	return (htmltext);
	}
function getThemeVersion () { //5/24/26 by DW
	const themeInfo = globals.myChatUserInterface.getThemeInfo ();
	return (themeInfo.version)
	}
function formatDatesForDisplay (theData) { //5/24/26 by Claude
	const isoDatePattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/;
	const result = {};
	Object.keys (theData).forEach (function (key) {
		const value = theData [key];
		if (typeof value === "string" && isoDatePattern.test (value)) {
			result [key] = new Date (value).toLocaleString ("en-US", {timeZoneName: "short"});
			}
		else {
			result [key] = value;
			}
		});
	return (result);
	}
function viewOutlineData (theData, title) { //5/24/26 by DW
	console.log ("viewOutlineData");
	function afterOpenCallback () {
		const formattedData = formatDatesForDisplay (theData);
		opInsertObject ("", formattedData, down); 
		opFirstSummit (); //blank line
		opDeleteLine ();
		opExpand ();
		opPromote ();
		opGo (down, infinity);
		opDeleteLine (); //delete curly brace
		opFirstSummit ();
		opDeleteLine (); //delete curly brace
		}
	const outlineOptions = {
		title,
		flReadOnly: true,
		outlineFontSize: 14,
		outlineLineHeight: 20,
		whereToAppend: $("body"), 
		divDialogStyles: "divDataDialog",
		opmltext: undefined,
		afterOpenCallback
		};
	outlineDialog (outlineOptions);
	}
function viewPrefsData () { //5/24/26 by DW
	console.log ("viewPrefsData");
	viewOutlineData (appPrefs, "User preferences, aka appPrefs.");
	}
function viewItemData (theData) { //5/24/26 by DW
	console.log ("viewItemData");
	viewOutlineData (theData, "This is the data we have for the current item.");
	}
function getScreenname () { //5/24/26 by DW
	const name = globals.myRssNetwork.getScreenname ();
	return (name);
	}
function getEmailAddress () { //5/27/26 by DW
	const email = globals.myRssNetwork.getEmail ();
	return (email);
	}
function applyUserCss (theCss) { //5/28/26 by DW
	const idStyle = "idUserCss";
	$("#" + idStyle).remove (); //clear any style we added on a previous call
	var theStyle = $("<style></style>");
	theStyle.attr ("id", idStyle);
	theStyle.text (theCss);
	theStyle.appendTo ("head");
	}
function httpRequest (url, timeout, headers, callback) { //cribbed from wordland -- 5/30/26 by DW 
	timeout = (timeout === undefined) ? 30000 : timeout;
	var jxhr = $.ajax ({ 
		url: url,
		dataType: "text", 
		headers,
		timeout
		}) 
	.success (function (data, status) { 
		callback (undefined, data);
		}) 
	.error (function (status) { 
		var message;
		try { //9/18/21 by DW
			message = JSON.parse (status.responseText).message;
			}
		catch (err) {
			message = status.responseText;
			}
		if ((message === undefined) || (message.length == 0)) { //7/22/22 by DW & 8/31/22 by DW
			message = "There was an error communicating with the server.";
			}
		var err = {
			code: status.status,
			message
			};
		callback (err);
		});
	}
function userCanEditTimelineItem () { //6/9/26 by DW
	const theItem = globals.myChatUserInterface.getCurrentItemInfo (); //6/24/26 by DW
	if (isLoggedIn ()) {
		if (theItem === undefined) { //6/24/26 by DW
			return (false);
			}
		else {
			if (globals.myRssNetwork.getScreenname () === theItem.screenname) { //6/24/26 by DW
				return (true);
				}
			else {
				return (false);
				}
			}
		}
	else {
		return (false);
		}
	}
function enableEyeIcon () { //6/9/26 by DW
	if (globals.currentTimelineItem === undefined) {
		return (false);
		}
	else {
		if (globals.currentTimelineItem.guid === undefined) {
			return (false);
			}
		else {
			return (true);
			}
		}
	}
function checkWithWhitelist (mailaddress, callback) { //6/10/26 by DW
	if (appConsts.flUseWhitelist) {
		if (globals.userData.flWhitelist) {
			globals.myRssNetwork.checkWhitelist (mailaddress, function (err, data) { 
				if (err) {
					callback (err);
					}
				else {
					callback (undefined, data.flWhitelisted);
					}
				});
			}
		else {
			callback (undefined, true);
			}
		}
	else {
		callback (undefined, true);
		}
	}
function isEmailInDatabase (email, callback) { //6/17/26 by DW -- glue for signupdialog.js
	globals.myRssNetwork.isEmailInDatabase (email, function (err, data) {
		if (err) {
			console.log ("isEmailInDatabase: err.message == " + err.message);
			callback (false);
			}
		else {
			callback (data.flInDatabase);
			}
		});
	}
function isUserInDatabase (screenname, callback) { //6/17/26 by DW
	globals.myRssNetwork.isUserInDatabase (screenname, function (err, data) {
		if (err) {
			console.log ("isUserInDatabase: err.message == " + err.message);
			callback (false);
			}
		else {
			callback (data.flInDatabase);
			}
		});
	}
function createNewUser (email, name, callback) { //6/17/26 by DW
	globals.myRssNetwork.createAccount (email, name, function (err, data) {
		if (err) {
			console.log ("createNewUser: err.message == " + err.message);
			callback (err);
			}
		else {
			console.log ("createNewUser: data == " + jsonStringify (data));
			callback (undefined, data);
			}
		});
	}
function getPermalinkUrl (id) { //6/25/26 by DW
	const theGuid = location.origin + "?id=" + id;
	return (theGuid);
	}
function setLastMessageRead (theItem) { //6/27/26 by DW
	var flSet = false;
	if (appPrefs.idLastMessageRead === undefined) { //6/27/26 by DW
		flSet = true;
		}
	else {
		if (theItem.id > appPrefs.idLastMessageRead) {
			flSet = true;
			}
		}
	if (flSet) {
		appPrefs.idLastMessageRead = theItem.id;
		prefsChanged ();
		}
	}
function fixPrefs () { //7/2/26 by DW
	if (appPrefs.currentThemeName == "twitter") {
		appPrefs.currentThemeName = "classic";
		prefsChanged ();
		}
	}
function firstCharUpper (s) { //7/7/26 by CC
	return (s.charAt (0).toUpperCase () + s.slice (1));
	}
function getGoodbyDialogMessage () { //7/17/26 by DW
	const theMessage = appConsts.productNameForDisplay + " is running in another tab. Click OK to reload this tab, or you can safely close it without losing any work.";
	return (theMessage)
	}
