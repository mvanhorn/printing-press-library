//04/26 2018 MIYU: Clean up order.base.dinein.js -> order.base.miyu.js -> order.base.mobile.js
//common code in order.base.dinein.js order.base.js removed all no longer used etc.

var restaurants;
var currentRest;
var context = {};
var timer;
var currentWeekDay = function () 
{
	return getWeekDay(new Date().getDay());
}

var scrollposition = 0;

//used in index.js renderWithLauncher
function renderRestrauntsPage() {	
	loadRestaurantsData(function(data){
		for (var i = restaurants.length - 1; i >= 0; i--) {
			restaurants[i].opened = inOpenHours(restaurants[i]);
		};

		renderHomeHeader();
		fixTopMergin();
		
		$("#container").html($.templates.restaurants.render({ restaurants: restaurants, isMobile: context.isMobile }));
		var lastOrder = getLastOrder();
		onRestaurantsLoaded();
	})
}

//this file only, used in renderRestaurantsPage
function renderHomeHeader() {
	var lastOrder = getLastOrder();
	var txtHeaderLine2;
	var txtHeaderLine3;
	if (restaurants && restaurants.length > 0)
	{
		txtHeaderLine2 = restaurants[0].fullname;			// hack, if more than 1 restaurants, display info in header from the first restaurant
		txtHeaderLine3 = restaurants[0].description;
	} else
	{
		txtHeaderLine2 = "Under maintenance";
		txtHeaderLine3 = "The service will be back online soon";
	}
	var ctx = {
		hasOrder : (lastOrder != null) && (restaurants && restaurants.length > 0),
		//txtHeaderLine1: 'Public Beta',
		txtHeaderLine2: txtHeaderLine2,
		txtHeaderLine3: txtHeaderLine3
	}
	$("#headerContainer").html($.templates.homeheader.render(ctx));

	var lastOrder = getLastOrder();
	$('#trackOrder').tap(function () {
    	if (typeof lastOrder == 'undefined' || lastOrder == null) {
    		// TODO: allow input order number
    		setTimeout(function(){ // 6/3/2016: hack, on iOS, the "track order" button doesn't seem to pop up after being pressed down if the alert is called immediately, so need to do alert after the button pops up 
    			alert("You have not placed any togo order yet.");
    		}, 200);
    	}
    	else {
			currentRest = getRestaurantByInfo(lastOrder.restname, lastOrder.restid);
			var orderToken = lastOrder.orderid;
			$(window).scrollTop(0);
			History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
    	}
	});
}

function getLastOrder() {
	var orderHistory = getOrderToGoHistory();
	if (orderHistory == null) {
		return null;
	} else {
		//now use orderToken replace orderid
		//by the way keep simple, so retain key

		var token = orderHistory[0].orderToken;
		var restname = token.substring(0, token.indexOf("_"));

		return {orderid: token, restid: orderHistory[0].order.restid, restname : restname};
	}
}


//04/25 2018 MIYU: Clean up order.base.js
//moved all common functions to here and remove duplicated code in
// order.mobile.dinin.js and order.mobile.togo.js


context.isMobile = true;
// Exclude "a" in the excludedElements
$.fn.swipe.defaults.excludedElements = "label, button, input, select, textarea, .noSwipe";


// common code for order.base.js & order.mobile.js 

var slideIndex = 1;
var curFilter = 1;

//setTimeout to fix stupid safari bugs, only used in the file
function fixSafari_float(){
	setTimeout(function(){
		$(".category-scroll-container .col").css("float","right");
		setTimeout(function(){
			$(".category-scroll-container .col").css("float","left");
		}, 200);					
	}, 200);
}

//common
function backToOriginalScroll(){
	//scroll >=200
	$(".header-banner").removeClass('header-down');
	$(".header-banner").removeClass("header-up");
	$(".header-banner").addClass('header-up-nomove');
}

//only used in m.togo
function fixTopMergin(){
	$("#container").css("margin-top","0px");
}

var categories_clicked = false;
var onLevelTwoCategory = false;
var scrollLevTwoPageSaver = 0;
var scrollSearchPageSaver = 0;
var scrollAllPageSaverForSafari = 0;
var onSearchPage = false;
var disableHeaderMove = false;
function onMenuLoaded(onSearchResult) {
	if(onSearchResult != true)
	{
		// the backgroud-color is not good for big screen device
		$("#mainbody-Container").css("background-color","");

		disableHeaderMove = false;
		// every keypress will make time reset.
		var userInputingTime;
		
		$("#search_input").bind('input porpertychange keypress',function(){
			clearTimeout(userInputingTime);
			userInputingTime = setTimeout(() => {
				categories_clicked = true;
				searchInputTapped();

				$('.S-dishInfo').tap(function(){
					dishInfoTapped(this);
				});	
			}, 500);
		});

		$('#searchBtn').tap(function(){
			// load chs_cht.js
			if (document.querySelector('script[src="/javascripts/chs_cht.js"]') == null) {
				loadScript("/javascripts/chs_cht.js", showSearchAfterLoadScript);
			}
			else
				showSearchAfterLoadScript();
			function showSearchAfterLoadScript() {
				onSearchPage = true;
				scrollSearchPageSaver = $(window).scrollTop();
				searchBtnTapped();
			}
		});

		$('#search_close').tap(function(){
			onSearchPage = false;
			// hide search layout and show dishes menu
			afterShowSearchResult();
			$(window).scrollTop(scrollSearchPageSaver);
		});

		var categoriesWidth = calCategoriesWidth();
		$(window).on("touchmove mousewheel",function(){
			categories_clicked = false;
			$("#search_input").blur();
			$('.header-banner').removeClass('header-down-nomove');
			window.timeLastScroll = (new Date()).getTime();
		});

		if($('.twoLevel-ctgyname').is(":visible"))
		{
			onLevelTwoCategory = true;
			hideElemsBeforeShowTwoLevelCategory();
		}

		$(window).scroll(function(){
			var top=document.documentElement.scrollTop || document.body.scrollTop;		
			var curId = "";	
			var items = $('.listviewOverlay.menuitem-Container');
			var isList = true;
			var appied = false;
			if(items.length === 0)
			{
				items = $('.dishNameHolder-Container');
				isList = false;
			}
			items.each(function(){						
				var itemsTop = $(this).offset().top-370;	
				//console.log("top:"+top+" itemsTop:"+itemsTop+ " ="+(itemsTop-top));	
				function checkIfHighlight(cur_cate,categories_clicked,categoriesWidth,category){
					if(!$(cur_cate).find('a').hasClass("highlighted") && categories_clicked === false){
						//removefilter();
						remove_highlightbar();
						apply_highlightbar(cur_cate);
						var left = categoriesWidth[category][1]-$(window).width()/2-categoriesWidth[category][0]/2;
						$('.category-scroll').animate({scrollLeft:left},150,"linear");
					}
				}
				if(isList)
				{
					if(top-210 > itemsTop && top-250 < itemsTop) //240 280
					{
						curId = "#" + $(this).attr("id");
						var str = $(curId).find(".dishNameHolder-Container")[0].innerText;
						var category = $(curId).find(".dishInfo-Container").attr("category");
						var cur_cate = $('#scrollbar-nav [category="' + category + '"]');
						// check if remove_highlightbar / apply_highlightbar rgb(47, 47, 47) === #2f2f2f
						checkIfHighlight(cur_cate,categories_clicked,categoriesWidth,category);
						return false;
					}
				}else
				{
					itemsTop = $(this).parent().parent().offset().top-375;
					if(top-210 > itemsTop && top-250 < itemsTop)
					{
						var category = $(this).parent().attr("category");
						var cur_cate = $('#scrollbar-nav [category="' + category + '"]');
						checkIfHighlight(cur_cate,categories_clicked,categoriesWidth,category);
						return false;
					}
				}
				// break items.each after appied
				if(appied)
					return false;				
			});				
		});

		$(".twoLv-backbtn").tap(function(){
			$(window).scrollTop(scrollLevTwoPageSaver);
			onLevelTwoCategory = true;
			hideElemsBeforeShowTwoLevelCategory();
			$(".twoLevel-ctgyname").show();
			if($(window).scrollTop()<=200)
				$('.jumbotron_restaurant').show();
		});

		// two level category click
		$(".twoLevel-ctgyname.category").tap(function()
		{
			twoLevelCtgyTapped(this, categoriesWidth);
		});
	}
	$(document.body).css("max-width", "").css("margin", "");

	// after switch list/view
	$(".filter").tap(function(){
		categories_clicked = true;
		remove_highlightbar();
		apply_highlightbar(this);
		var l_this_attr_name = this.attributes["category"].value;
		var categories = $('.menu-container').find('.S-category-name'); //$('.S-category-name');
		function scrollToDishFromFilter(itemTop,category,l_this_attr_name,offset)
		{
			if(l_this_attr_name.replace(/\s/g, "") === category.replace(/\s/g, "")){
				itemTop = (itemTop - 111) + offset;
				$("html, body").animate({ scrollTop: itemTop+'px' });
				return false;
			}
		}
		categories.each(function(){		
			var itemTop = $(this).offset().top;
			var category = $(this).find('span')[0].innerText;
			return scrollToDishFromFilter(itemTop,category,l_this_attr_name,0);
		});
		$('.header-banner').removeClass('header-down-nomove');
	});
	$(".menuitem-Container").tap(function(){
		var currentTime = (new Date()).getTime();
		if (currentTime - window.timeLastScroll < 500)
		{
			return false;
		}
		var target = this;
		disableHeaderMove = saveScrollandloadDetails(target, disableHeaderMove);		
	});

	$('#openHoursToday').tap(function () {
		showRestaurantOpenSchedule();
	});

	$(".addToCart").tap(function () {
		addToCartTapped(this, disableHeaderMove);
	});

	$("#viewCart").tap(viewCartClicked);

	window.goBackHome = function goBackHome()
	{
		if (context.tableId && getQueryParams(document.location.search).tid == null)
			window.location="/?cover=" + context.rest.name + "&secm=1";
		else if (context.tableId && getQueryParams(document.location.search).tid)
			window.location = "/?cover=" + context.rest.name + "&tid=" + getQueryParams(document.location.search).tid;
		else
			window.location="/";
	}
	$("#backToHome").tap(function(){
		window.goBackHome();
	});

	//setupDishOptions();	
	$(window).scroll(function(){
		if(onLevelTwoCategory)
		{
			$(".header-banner").removeClass('header-down header-up');
			if ($(window).scrollTop() >= 200) {	
				$(".header-banner").removeClass('header-down-withImg');
				fixSafari_float();
				if(disableHeaderMove){
					$(".header-banner").removeClass('header-up-withImg').addClass('header-up-nomove-withImg');
					disableHeaderMove = false;
				} else {
					$(".header-banner").addClass('header-up-withImg');
				}
			}
			else {
				$('.jumbotron_restaurant').show();
				if(!disableHeaderMove){
					//prevent slide when back from details page
					if($(".header-banner").hasClass('header-up-nomove-withImg')){
						$(".header-banner").removeClass('header-up-nomove-withImg').addClass('header-up-withImg');
					} else {
						$(".header-banner").removeClass('header-up-withImg').addClass('header-down-withImg');
					}
				} 
			}
		}else
		{
			// old path
			$(".header-banner").removeClass('header-down-withImg header-up-withImg');
			if ($(window).scrollTop() >= 15) {	
				$(".header-banner").removeClass('header-down');
				fixSafari_float();
				if(disableHeaderMove){
					$(".header-banner").removeClass('header-up').addClass('header-up-nomove');
					disableHeaderMove = false;
				} else {
					$(".header-banner").addClass('header-up');
				}
			}
			else {
				if(!disableHeaderMove){
					//prevent slide when back from details page
					if($(".header-banner").hasClass('header-up-nomove')){
						$(".header-banner").removeClass('header-up-nomove').addClass('header-up');
					} else {
						$(".header-banner").removeClass('header-up').addClass('header-down');
					}
				} 
			}
		}
	});
}

function hideElemsWhenMenuDetailShows(){
	if(onSearchPage)
	{
		$('#home-header-3rd').hide();
		$('#searchlay').hide();
		//$('#home-header-2nd').hide();
		$('#viewCartBtn').hide();
	}else
	{
		$('.waterfall').hide();
		$('.header-banner').hide();
		$('#home-header-2nd').hide();
		$('#viewCartBtn').hide();
	}
}

function showElemsWhenMenuDetailShows(){
	if(onSearchPage)
	{
		$('#home-header-3rd').show();
		$('#searchlay').show();
		//$('#home-header-2nd').show();
		$('#viewCartBtn').show();
	}else
	{
		$('.waterfall').show();
		$('.header-banner').addClass('header-up-nomove');
		$('.header-banner').show();
		$('#home-header-2nd').show();
		$('#viewCartBtn').show();
	}
}

function beforeShowSearchResult(){
	$('#home-header-3rd').css("display","");
	$('#home-header-2nd').hide();
	$('#searchlay').show();
	//$('#viewCartBtn').hide();
	$('.waterfall').hide();
	$('.header-banner').hide();
	$('#container').css('height','0px');
	//$('#mainbody-Container').css('background-color','');
}

function afterShowSearchResult(){
	$('#home-header-3rd').css("display","none");
	$('#home-header-2nd').show();
	$('#searchlay').hide();
	$('#viewCartBtn').show();
	$('.waterfall').show();
	$('.header-banner').show();
	$('#container').css('height','100%');
	//$('#mainbody-Container').css('background-color','#f7f7f7');
}

// after click category on two level
function showElemsAfterShowTwoLevelCategory(){
	$('.menu-toolbar').show();
	$(".category-scroll-container").show();
	$('.jumbotron_restaurant').hide();
	$('.twoLevel-ctgyname').hide();
	$('#home-header').css("display","none");
	$('#home-header-2nd').css("display","");
	$('#viewCartBtn').show();
}

// click back btn or first into page
function hideElemsBeforeShowTwoLevelCategory(){
	$('#home-header').css("display","");
	$('#home-header-2nd').css("display","none");
	$(".category-scroll-container").hide();
	$(".waterfall").hide();
	$('.menu-toolbar').hide();
	//$('#viewCartBtn').hide();
	$(".header-banner").removeClass('header-up-nomove');
}

//all dif
function closeMenuDetails(){
	var scrollposition = $("#item_details_page").attr("scrollposition");
	$("#container").css("display", "");
	$("#headerContainer").css("display", "");
	$(window).scrollTop(scrollposition);
	$(".container-fluid").css("overflow-y", "unset");
	
	setTimeout(() => {
		$("#item_details_page").empty();
		$("#item_details_page_container").addClass("hide-banner");
		$("#item_details_page_filter").addClass("hide-banner");
	}, 50);
	
	$(window).scrollTop(scrollposition);

	if (scrollposition > 200) {
		// //scroll >=200
		backToOriginalScroll();
	} else {
		//has hear-up-no-move
	}
	enableBodyScroll();
	showElemsWhenMenuDetailShows();
	// enable input box for big screen chrome pad when close MenuDetail Info
	setTimeout(() => {
		$("#search_input").prop('disabled', false);
	}, 200);
}