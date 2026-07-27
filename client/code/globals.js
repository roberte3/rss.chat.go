const appConsts = {
	version: "0.6.12", 
	
	productName: settingsFromServer.productName,
	productNameForDisplay: settingsFromServer.productNameForDisplay,
	flEnableLogin: settingsFromServer.flEnableLogin,
	urlServer: settingsFromServer.urlServer,
	flSocketsEnabled: getBoolean (settingsFromServer.flSocketsEnabled), //7/20/26 by DW
	urlSocketServer: settingsFromServer.urlSocketServer,
	maxCharsItemText: 100,
	flEllipsesAfterText: true,
	flIncludeAllSocketItems: false,
	flPostItemsLocally: false, //4/24/26 by DW
	maxSecsForIds: 60, 
	urlDefaultImage: "https://imgs.scripting.com/2024/09/10/kittyStamp.png", //5/4/26 by DW
	userData: new Object (), //5/17/26 by DW
	urlThemes: "//s3.amazonaws.com/scripting.com/code/rsschat/themes/", //7/2/26 by DW
	flUseWhitelist: true, //6/9/26 by DW
	}

var appPrefs = { //5/16/26 by DW
	ctStarts: 0,
	ctMinutes: 0,
	whenLastStart: new Date (),
	lastEmail: undefined, 
	currentThemeName: "classic", //7/2/26 by DW
	savedUserDraft: undefined, //5/18/26 by DW
	flWordCount: false,
	lastUrlString: undefined,
	lastTitleString: undefined,
	defaultEditorMode: "wizzy", //5/19/26 by DW
	placeholderSignedIn: "What's happening?", //5/24/26 by DW
	placeholderSignedOut: "Sign in to post",
	placeholderReply: "Post your reply here.",
	myFeedTitle: undefined, //5/27/26 by DW
	myFeedLink: undefined,
	myFeedDescription: undefined,
	myAvatarImageUrl: undefined, //6/1/26 by DW
	flApplyUserCss: false, //5/28/26 by DW
	idLastMessageRead: undefined, //6/27/26 by DW
	userCss: [
		":root {",
		"\t--text-size: 16px;",
		"\t--text-color: #0f1419;",
		"\t--line-spacing: 1.4;",
		"\t--title-size: 18px;",
		"\t--link-color: rgb(30, 104, 166);",
		"\t--font: Ubuntu, system-ui, sans-serif;",
		"\t--text-display-width: 640px;",
		"\t}"
		] .join ("\n"),
	};

var globals = {
	myRssNetwork: undefined,
	myChatUserInterface: undefined, 
	myFirehoseSocket: undefined,
	lastFeedUrl: null, //should be undefined
	lastDateStr: null, 
	currentTimelineItem: undefined, //6/1/26 by DW
	}

var flPrefsChanged = false;
var myTurndown = undefined;

const leftColumnIcons = [ //5/20/26 by DW
	{ //home
		name: "home",
		title: "Home",
		icon: "<i class=\"fas fa-home\"></i>",
		tooltip: "Click to go to the back to the timeline.",
		enabled: true, 
		click: function () {
			clickHomeButton (); //6/19/26 by DW
			}
		},
	{ //big plus
		name: "bigPlus",
		title: "New post",
		icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round">
			<line x1="8" y1="3" x2="8" y2="13" />
			<line x1="3" y1="8" x2="13" y2="8" />
			</svg>
			`,
		tooltip: "Add new post.",
		enabled: true, 
		click: function () {
			globals.myChatUserInterface.editNewItem (); //5/20/26 by DW
			}
		},
	{ //feed
		name: "feed",
		title: "Your feed",
		icon: "<i class=\"fas fa-rss\"></i>",
		tooltip: "View your " + appConsts.productNameForDisplay + " feed.",
		enabled: true,
		click: function (ev) {
			viewMyFeedCommand ();
			}
		},
	{ //view data
		name: "data",
		title: "Your prefs",
		icon: "<i class=\"fa fa-code\"></i>",
		tooltip: "View your prefs in JSON.",
		enabled: true,
		click: function (ev) {
			viewPrefsData ();
			}
		},
	
	];

const insideItemIcons = [ //5/22/26 by DW
	{ //comment
		name: "comment",
		icon: "<i class=\"far fa-comment fa-comment\"></i>",
		enabled: true, 
		tooltip: "Write a reply to this post.",
		click: function (ev) {
			console.log ("comment");
			globals.myChatUserInterface.editNewComment (ev.item);
			}
		},
	{ //heart icon -- 6/25/26 by DW
		name: "like",
		icon: "<i class=\"far fa-heart\"></i>",
		enabled: true, 
		click: function (ev) {
			console.log ("like");
			globals.myChatUserInterface.toggleLike (ev.item);
			}
		},
	{ //pencil icon
		name: "pencil",
		icon: "<i class=\"fas fa-edit\"></i>",
		enabled: true, 
		tooltip: "Edit this post.",
		click: function (ev) {
			console.log ("pencil");
			globals.myChatUserInterface.editItem (ev.item);
			}
		},
	{ //view data
		name: "data",
		icon: "<i class=\"fa fa-code\"></i>",
		tooltip: "View data for this item.",
		enabled: true, 
		click: function (ev) {
			viewItemData (ev.item);
			}
		},
	]

const editorMenu = [ //6/10/26 by DW
	{ //set title
		name: "setTitle",
		display: "Set title...",
		enabled: true, 
		tooltip: "Edit the title of the item.", 
		click: function (ev) {
			console.log ("edit");
			globals.myChatUserInterface.editTitle ();
			}
		},
	{ //view data
		name: "viewData",
		display: "View data...",
		enabled: true, 
		tooltip: "View the data we're storing for the item.", 
		click: function (ev) {
			console.log ("View data");
			const editorData = globals.myChatUserInterface.getEditorData ();
			viewItemData (editorData);
			}
		},
	]
const timelineItemMenu = [ //6/10/26 by DW
	{ //edit
		name: "edit",
		display: "Edit",
		enabled: true, 
		tooltip: "Edit the contents of this post.", 
		click: function (ev) {
			console.log ("edit");
			globals.myChatUserInterface.editItem (ev.item);
			}
		},
	{ //delete
		name: "delete",
		display: "Delete",
		enabled: true, 
		tooltip: "Delete this post.", 
		click: function (ev) {
			console.log ("delete");
			deletePostCommand (ev.item);
			}
		},
	{ //divider
		flDivider: true
		}, 
	{ //placeholder 1
		name: "placeholder1",
		display: "Placeholder #1",
		enabled: true, 
		tooltip: "Something useful will be here soon.", 
		click: function (ev) {
			console.log ("Placeholder #1");
			alertDialog ("This is just a placeholder while the app is in development. There will be commands here but they aren't ready yet.");
			}
		},
	{ //placeholder 2
		name: "placeholder2",
		display: "Placeholder #2",
		enabled: true, 
		tooltip: "Something useful will be here soon.", 
		click: function (ev) {
			console.log ("Placeholder #2");
			alertDialog ("This is just a placeholder while the app is in development. There will be commands here but they aren't ready yet.");
			}
		},
	]
