function rssNetworkServer (userOptions) {
	var rssNetworkMemory = { //saved in localStorage
		email: undefined,
		code: undefined,
		screenname: undefined
		};
	var options = {
		serverAddress: undefined
		};
	mergeOptions (userOptions, options);
	
	var myFeedUrl = undefined, baseFeedUrl = undefined;
	var flWillRedirect = false; //5/17/26 by DW -- will be true if we're redirecting when things settle down
	
	function saveMemory () {
		localStorage.rssNetworkMemory = JSON.stringify (rssNetworkMemory);
		}
	function loadMemory () {
		if (localStorage.rssNetworkMemory !== undefined) {
			var jstruct = JSON.parse (localStorage.rssNetworkMemory);
			for (var x in jstruct) {
				rssNetworkMemory [x] = jstruct [x];
				}
			}
		}
	function getLinkToAppPage () {
		return (stringNthField (stringNthField (window.location.href, "?", 1), "#", 1));
		}
	function handleEmailConfirm () {
		var params = new URLSearchParams (location.search);
		if (params.get ("emailconfirmed") === "true") {
			rssNetworkMemory.email = params.get ("email");
			rssNetworkMemory.code = params.get ("code");
			rssNetworkMemory.screenname = params.get ("screenname");
			saveMemory ();
			flWillRedirect = true; //5/17/26 by DW
			location.href = getLinkToAppPage ();
			}
		}
	function buildQueryString (params) {
		var result = "";
		if (params !== undefined) {
			for (var x in params) {
				if (params [x] !== undefined) {
					if (result.length > 0) {
						result += "&";
						}
					result += encodeURIComponent (x) + "=" + encodeURIComponent (params [x]);
					}
				}
			}
		return (result);
		}
	function servercall (path, params, flAuthenticated, method, callback) {
		method = (method === undefined) ? "GET" : method;
		var url = options.serverAddress + path;
		
		if (params === undefined) { //6/12/26 by DW
			params = new Object ();
			}
		if (getBoolean (flAuthenticated)) {
			params.emailaddress = rssNetworkMemory.email;
			params.emailcode = rssNetworkMemory.code;
			}
		
		var qs = buildQueryString (params);
		if (qs.length > 0) {
			url += "?" + qs;
			}
		fetch (url, {method}) .then (function (response) {
			return (response.text () .then (function (responseText) {
				if (response.ok) {
					try {
						const jstruct = JSON.parse (responseText);
						callback (undefined, jstruct);
						}
					catch (err) {
						callback (err);
						}
					}
				else {
					try {
						const jstruct = JSON.parse (responseText);
						callback (jstruct);
						}
					catch (err) {
						callback ({message: responseText});
						}
					}
				}));
			})
		.catch (function (err) {
			callback (err);
			});
		}
	
	function createAccount (email, name=undefined, callback) {
		var params = {
			email,
			name: (name == undefined) ? trimWhitespace (stringNthField (email, "@", 1)) : name,
			urlredirect: getLinkToAppPage ()
			};
		servercall ("createnewuser", params, false, "GET", callback); //this is in daveappserver -- 6/5/26 by DW
		}
	function signIn (email, callback) {
		var params = {
			email,
			urlredirect: getLinkToAppPage ()
			};
		servercall ("sendconfirmingemail", params, false, "GET", callback);
		}
	function getFeedUrl () {
		return (myFeedUrl);
		}
	function getBaseFeedUrl () {
		return (baseFeedUrl);
		}
	function getFeed (screenname, callback) {
		servercall ("feed", {screenname}, false, "GET", callback);
		}
	function getUserData (screenname, callback) {
		servercall ("getuserdata", {screenname}, true, "GET", callback);
		}
	function getSubscriptionList (callback) {
		servercall ("getsubscriptionlist", undefined, false, "GET", callback);
		}
	function newPost (postRec, callback) {
		if (userIsSignedIn ()) {
			const params = {
				jsontext: jsonStringify (postRec),
				};
			servercall ("newpost", params, true, "POST", callback);
			console.log (nowstring () + " rssNetworkServer.newPost: postRec == " + jsonStringify (postRec)); //5/14/26 by DW
			}
		else {
			const message = "Can't send a new post because you are not signed in.";
			callback ({message});
			}
		}
	function updatePost (postRec, callback) { //5/21/26 by DW
		if (userIsSignedIn ()) {
			const params = {
				jsontext: jsonStringify (postRec),
				};
			servercall ("updatepost", params, true, "POST", callback);
			console.log (nowstring () + " rssNetworkServer.updatePost: postRec == " + jsonStringify (postRec));
			}
		else {
			const message = "Can't send a new post because you are not signed in.";
			callback ({message});
			}
		}
	function deletePost (id, callback) { //5/21/26 by DW
		if (userIsSignedIn ()) {
			servercall ("deletepost", {id}, true, "POST", callback);
			}
		else {
			const message = "Can't delete the post because you are not signed in.";
			callback ({message});
			}
		}
	function getVersion (callback) {
		servercall ("version", undefined, false, "GET", callback);
		}
	function userIsSignedIn () {
		return ((rssNetworkMemory.email !== undefined) && (rssNetworkMemory.code !== undefined));
		}
	function getSocketGreeting () { //7/17/26 by CC 
		if (userIsSignedIn ()) {
			return ("user " + rssNetworkMemory.email + " " + rssNetworkMemory.code);
			}
		else {
			return (undefined);
			}
		}
	function getScreenname () {
		return (rssNetworkMemory.screenname);
		}
	function getEmail () {
		return (rssNetworkMemory.email);
		}
	function signOut () {
		delete localStorage.rssNetworkMemory;
		location.reload ();
		}
	function getRecentItems (ct, callback) { //4/29/26 by DW
		const screenname = getScreenname (); //6/24/26 by DW
		servercall ("getrecentitems", {screenname, ct}, false, "GET", callback);
		}
	function savePrefs (thePrefs, callback) { //5/16/26 by DW
		if (userIsSignedIn ()) {
			const params = {
				jsontext: jsonStringify (thePrefs),
				};
			servercall ("saveprefs", params, true, "POST", callback);
			}
		else {
			const message = "Can't save prefs because you are not signed in.";
			callback ({message});
			}
		}
	function willRedirect () { //5/17/26 by DW
		return (flWillRedirect);
		}
	function getItemByGuid (guid, callback) { //6/8/26 by DW
		if (guid == undefined) {
			const message = "Can't get the item record because the GUID param is undefined.";
			console.log ("getItemByGuid: " + message);
			console.trace ();
			callback ({message});
			}
		else {
			const screenname = getScreenname (); //6/24/26 by DW
			servercall ("getitembyguid", {screenname, guid}, false, "GET", callback);
			}
		}
	function checkWhitelist (emailaddress, callback) { //6/9/26 by DW
		servercall ("checkwhitelist", {emailaddress}, false, "GET", callback);
		}
	function isUserInDatabase (screenname, callback) { //6/15/26 by DW
		servercall ("isuserindatabase", {screenname}, true, "GET", callback);
		}
	function isEmailInDatabase (email, callback) { //6/15/26 by DW
		servercall ("isemailindatabase", {email}, true, "GET", callback);
		}
	function toggleLike (id, callback) { //6/24/26 by DW
		if (userIsSignedIn ()) {
			servercall ("togglelike", {id}, true, "POST", callback);
			}
		else {
			const message = "Can't like something because you are not signed in.";
			callback ({message});
			}
		}
	function getLikersList (id, callback) { //6/25/26 by CC
		servercall ("getlikerslist", {id}, false, "GET", callback);
		}
	function getRecentUserItems (name, callback) { //6/26/26 by DW
		const screenname = getScreenname ();
		servercall ("getrecentuseritems", {screenname, name}, false, "GET", callback);
		}
	function getItemAndReplies (screenname, idparent, callback) { //6/30/26 by DW
		servercall ("getitemandreplies", {screenname, idparent}, false, "GET", callback);
		}
	function getMostActiveToday (callback) { //7/1/26 by DW
		servercall ("getmostactivetoday", undefined, false, "GET", callback);
		}
	function uploadMedia (type, base64text, callback) { //7/22/26 by CC -- #188
		if (userIsSignedIn ()) {
			const params = {
				type,
				emailaddress: rssNetworkMemory.email,
				emailcode: rssNetworkMemory.code
				};
			const url = options.serverAddress + "uploadmedia?" + buildQueryString (params);
			fetch (url, {method: "POST", body: base64text}) .then (function (response) {
				return (response.text () .then (function (responseText) {
					if (response.ok) {
						try {
							const jstruct = JSON.parse (responseText);
							callback (undefined, jstruct);
							}
						catch (err) {
							callback (err);
							}
						}
					else {
						try {
							const jstruct = JSON.parse (responseText);
							callback (jstruct);
							}
						catch (err) {
							callback ({message: responseText});
							}
						}
					}));
				})
			.catch (function (err) {
				callback (err);
				});
			}
		else {
			const message = "Can't upload the image because you are not signed in.";
			callback ({message});
			}
		}
	
	function start (callback) { //async stuff we do at start -- 4/22/26 by DW
		console.log ("rssNetworkServer.start");
		getUserData (rssNetworkMemory.screenname, function (err, userData) {
			if (err) {
				console.log ("start: err.message == " + err.message);
				callback (err);
				}
			else {
				console.log ("start: userData == " + jsonStringify (userData));
				myFeedUrl = userData.feedUrl;
				baseFeedUrl = userData.baseFeedUrl;
				callback (undefined, userData);
				}
			});
		}
	
	loadMemory ();
	handleEmailConfirm ();
	
	console.log ("rssNetworkServer: rssNetworkMemory == " + jsonStringify (rssNetworkMemory));
	
	this.createAccount = createAccount; //4/20/26 by DW
	this.signIn = signIn; //4/19/26 by DW
	this.newPost = newPost;
	this.getVersion = getVersion;
	this.getFeed = getFeed;
	this.getFeedUrl = getFeedUrl;
	this.userIsSignedIn = userIsSignedIn;
	this.getScreenname = getScreenname;
	this.getEmail = getEmail;
	this.getBaseFeedUrl = getBaseFeedUrl; //4/23/26 by DW
	this.getSubscriptionList = getSubscriptionList; 
	this.getRecentItems = getRecentItems; //4/29/26 by DW
	this.savePrefs = savePrefs; //5/16/26 by DW
	this.willRedirect = willRedirect; //5/17/26 by DW
	this.updatePost = updatePost; //5/21/26 by DW
	this.getItemByGuid = getItemByGuid; //6/8/26 by DW
	this.checkWhitelist = checkWhitelist; //6/9/26 by DW
	this.deletePost = deletePost; //6/12/26 by DW
	this.getUserData = getUserData; //6/15/26 by DW
	
	this.isUserInDatabase = isUserInDatabase; //6/15/26 by DW
	this.isEmailInDatabase = isEmailInDatabase; //6/15/26 by DW
	
	this.toggleLike = toggleLike; //6/24/26 by DW
	this.getLikersList = getLikersList; //6/25/26 by DW
	
	this.getRecentUserItems = getRecentUserItems; //6/26/26 by DW
	this.getItemAndReplies = getItemAndReplies; //6/30/26 by DW
	
	this.getMostActiveToday = getMostActiveToday; //7/1/26 by DW
	this.getSocketGreeting = getSocketGreeting; //7/17/26 by CC
		this.uploadMedia = uploadMedia; //7/22/26 by CC -- #188
	
	this.signOut = signOut;
	this.start = start; //4/22/26 by DW
	}

function testGetSubscriptionList () {
	globals.myRssNetwork.getSubscriptionList (function (err, theList) {
		if (err) {
			console.log ("testGetSubscriptionList: err.message == " + err.message);
			}
		else {
			console.log ("testGetSubscriptionList: theList == " + jsonStringify (theList));
			}
		});
	}
function testCreateAccount () {
	const email = "dave.wine..r@gmail.com", name = "Cookie Magoo";
	globals.myRssNetwork.createAccount (email, name, function (err, data) {
		if (err) {
			console.log ("testCreateNewUser: err.message == " + err.message);
			}
		else {
			console.log ("testCreateNewUser: data == " + jsonStringify (data));
			}
		});
	}
function testSignIn () {
	const email = "dave.winer@gmail.com";
	globals.myRssNetwork.signIn (email, function (err, data) {
		if (err) {
			console.log ("testCreateNewUser: err.message == " + err.message);
			}
		else {
			console.log ("testCreateNewUser: data == " + jsonStringify (data));
			}
		});
	}
function testNewPost () {
	const postRec = {
		title: undefined, 
		description: "The time right now == " + new Date ().toLocaleString (), 
		inReplyTo: 12.
		}
	globals.myRssNetwork.newPost (postRec, function (err, data) {
		if (err) {
			console.log ("testNewPost: err.message == " + err.message);
			}
		else {
			console.log ("testNewPost: data == " + jsonStringify (data));
			}
		});
	}
function testGetVersion () {
	globals.myRssNetwork.getVersion (function (err, theVersion) {
		if (err) {
			console.log ("testGetVersion: err.message == " + err.message);
			}
		else {
			console.log ("testGetVersion: theVersion == " + jsonStringify (theVersion));
			}
		});
	}
function testGetFeed () {
	const screenname = globals.myRssNetwork.getScreenname ();
	globals.myRssNetwork.getFeed (screenname, function (err, xmltext) {
		if (err) {
			console.log ("testGetFeed: err.message == " + err.message);
			}
		else {
			console.log ("testGetFeed: xmltext == \n" + xmltext);
			}
		});
	}
function testGetFeedUrl () {
	console.log ("testGetFeedUrl: feedUrl == " + globals.myRssNetwork.getFeedUrl ());
	}
function testUserIsSignedIn () {
	const flSignedIn = globals.myRssNetwork.userIsSignedIn ();
	console.log ("testUserIsSignedIn: flSignedIn == " + flSignedIn);
	}
function testGetScreenname () {
	const screenname = globals.myRssNetwork.getScreenname ();
	console.log ("testGetScreenname: screenname == " + screenname);
	}
function testGetEmailAddress () {
	const email = globals.myRssNetwork.getEmail ();
	console.log ("testGetEmailAddress: email == " + email);
	}
function testSignOut () {
	globals.myRssNetwork.signOut ();
	}
function testGetRecentItems () {
	const ct = 175;
	globals.myRssNetwork.getRecentItems (ct, function (err, theItems) {
		if (err) {
			console.log ("testGetRecentItems: err.message == " + err.message);
			}
		else {
			console.log ("testGetRecentItems: theItems == \n" + jsonStringify (theItems));
			}
		});
	}
function testSaveprefs () {
	appPrefs.slogan = getRandomSnarkySlogan ();
	globals.myRssNetwork.savePrefs (appPrefs, function (err) {
		if (err) {
			console.log ("testSaveprefs: err.message == " + err.message);
			}
		else {
			console.log ("testSaveprefs: it worked, no data returned.");
			}
		});
	}
function testUpdatePost () { //5/21/26 by DW
	function getTimeString (theDate) {
		return (theDate.toLocaleTimeString ([], {hour: "2-digit", minute: "2-digit"}));
		}
	const postRec = {
		id: 115,
		title: getTimeString (new Date ()),
		description: "Good morning sports fans. This is a post that i have updated after I published it. The first of many I hope! :-)"
		};
	globals.myRssNetwork.updatePost (postRec, function (err, data) {
		if (err) {
			console.log ("testUpdatePost: err.message == " + err.message);
			}
		else {
			console.log ("testUpdatePost: data == " + jsonStringify (data));
			}
		});
	}
function testGetItemByGuid () { //6/8/26 by DW
	const guid = "https://rss.network/permalink?screenname=dave.winer&id=8pmie7h1";
	globals.myRssNetwork.getItemByGuid (guid, function (err, theItem) {
		if (err) {
			console.log ("testGetItemByGuid: err.message == " + err.message);
			}
		else {
			console.log ("testGetItemByGuid: theItem == \n" + jsonStringify (theItem));
			}
		});
	}
function testCheckWhitelist (screenname="hello@world.com") { //6/10/26 by DW
	globals.myRssNetwork.checkWhitelist (screenname, function (err, data) { 
		if (err) {
			console.log ("startPackages: err.message == " + err.message);
			}
		else {
			console.log ("startPackages: screenname == " + screenname + ", flWhitelisted == " + data.flWhitelisted);
			}
		});
	}
function testDeletePost (id=170) { //6/12/26 by DW
	globals.myRssNetwork.deletePost (id, function (err, data) { 
		if (err) {
			console.log ("testDeletePost: err.message == " + err.message);
			}
		else {
			console.log ("testDeletePost: data == " + jsonStringify (data));
			}
		});
	}
function testToggleLike (id=58) { //6/24/26 by DW
	globals.myRssNetwork.toggleLike (id, function (err, data) { 
		if (err) {
			console.log ("testToggleLike: err.message == " + err.message);
			}
		else {
			console.log ("testToggleLike: data == " + jsonStringify (data));
			}
		});
	}
function testGetMostActiveToday () { //7/1/26 by DW
	globals.myRssNetwork.getMostActiveToday (function (err, data) { 
		if (err) {
			console.log ("testGetMostActiveToday: err.message == " + err.message);
			}
		else {
			console.log ("testGetMostActiveToday: data == " + jsonStringify (data));
			}
		});
	}


