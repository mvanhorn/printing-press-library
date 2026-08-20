// 9/7/2019 miyu: Clearn up for 3rd Gen. This file contains all the common code used in menu realated mobile user page 
// include  dinein / mesh / table 

function removefilter(){
	var items = $(".menuitem");
	for (var i = 0; i < items.length; ++i) {			
		$(items[i]).removeClass("hidden");
	}
} 

function getRestaurantTable(restname, callback) {
	$.get('/m/api/restaurants/' + restname + '/tables').done(function(data){
		// var arr = [];
		// for (var k in data) {
		// 	if (
		// 		data.hasOwnProperty(k) &&
		// 		data[k] !== "Order To Go" &&
		// 		data[k] !== "Fantuan Order" &&
		// 		data[k] !== "Panda Order" &&
		// 		data[k] !== "Doordash Order" &&
		// 		data[k] !== "UberEats Order" &&
		// 		data[k] !== "NextDish Order"
		// 	) {
		// 		arr.push(data[k]);
		// 	}
		// }
		// data = arr;
		callback(data);
	});
}

function getRestaurantByInfo(restname, restid) {
	for (var i = restaurants.length - 1; i >= 0; i--) {
		if (restaurants[i].name == restname && restaurants[i].id == restid)
		{
			return restaurants[i];
		}
	}
	console.log("invalid rest restname - " + restname);
	$(".circular-loader").hide();
	$(".msgLabel").text("Online ordering is currently unavailable. Please try again later.");
	$('.msgLabel').first().appendTo('.profile-main-loader').first();
	$(".profile-main-loader").css("width","220px");
	$(".loader").hide();
	return null;
}

function renderMenuPageDirect(restname, restid, category, tableId, bPostTable) {
	// _restoreLocalStore();	
	loadRestaurantsData(function(data){
		var rest = getRestaurantByInfo(restname, restid);
		if (window.changeRestConfig) window.changeRestConfig(rest);
		if (bPostTable && (tableId == null || typeof tableId == 'undefined'))
		{
			var tableArr = getRestaurantTable(restname, function(data){
				renderMenuPage(rest, category, tableId, data);
			});
		} else
		{
			renderMenuPage(rest, category, tableId, null);
		}
	});
}

function checkCartHasItems(){
	var viewCartBtn = $("#viewCartBtn");
	var numItemBtn = $("#numItems");
	if (viewCartBtn)
	{		
		if(context.cart.items.length == 0){
			if (!window.isSTO) {
				viewCartBtn.removeClass("cartButton-contianer");
				viewCartBtn.addClass("cartButton-contianer-disabled");
			}
			numItemBtn.removeClass("numItems-active");
			numItemBtn.addClass("numItems-disabled");
			$("#clearAllItemsInCart").css("display","none");
			$(".dinein-ordersummary-table").css("display","none");
		} else {
			viewCartBtn.removeClass("cartButton-contianer-disabled");
			numItemBtn.removeClass("numItems-disabled");
			numItemBtn.addClass("numItems-active");
			viewCartBtn.addClass("cartButton-contianer");
			$("#clearAllItemsInCart").css("display","block");
		} 
	}
}

//highlight required option table if it is not selected with scroll up 
//only for mobile since option details page is vertical
function highLightRequiredOptions(option_name){
	var selected = $('.uv_optionsTable-holder table[option_name="' + option_name + '"] thead tr');
	var container = $('#item_details_page_container');

	//need to scroll to top before frash
	if(!isScrolledIntoView(selected, container)){
		var elemTop = $(selected).offset().top;
		var containerOffset = $('.uv_optionsTable-holder').offset().top;
		var scrollposition = Math.abs(containerOffset -  elemTop);
		scrollposition += 400;
		if ($("#item_details_page #item_image").css("display") == "none") scrollposition -= 450;
		container.animate({scrollTop: scrollposition}, 400, 'swing');
		setTimeout( function(){
			//wait scroll, then shake
			shakeDomObject(selected);
		}, 300);
	} else {
		shakeDomObject(selected);
	}	
}

function smoothScrollToMiddleOfSreen(el, callback)
{
	var elOffset = el.offset().top;
	var elHeight = el.height();
	var windowHeight = $(window).height();
	var offset;
	if (elHeight < windowHeight) {
		offset = elOffset - ((windowHeight / 2) - (elHeight / 2));
	}
	else {
		offset = elOffset;
	}
	$('#mainbody-Container').animate({scrollTop:offset}, 400, 'swing', callback);
}

function shakeDomObject(obj)
{
	smoothScrollToMiddleOfSreen(obj, function(){
		obj.addClass("shakeClass");
		setTimeout(function(){
			obj.removeClass("shakeClass");
			obj.focus();
		}, 400);
	});	
}

function remove_highlightbar(){
	$(".category_name a").css("color", "#B5B5B5");
	$(".category_name").css("border-bottom", "none");
	$(".category_name a").removeClass("highlighted");
}

// get category tags in scrollbar positions, key as the category, 
// [0] has the individual width, [1] has the outerwidth total
function calCategoriesWidth()
{
	var dict = {};
	var total = 0;
	$('#scrollbar-nav div').each(function(){
		total += $(this).outerWidth();
		dict[$(this).attr("category")] = [$(this).outerWidth(),total];
	});
	return dict;
}

//changed for dinein order...
function drawSummaryCart()
{
	sortAndMergeMenuItems(context.cart);
	var container = $('#modalConfirm');
	var ctx = {
		title: "Order  Summary",
		cart: context.cart,
		rest: context.rest,
		totals: {},
	};
	$.templates.dineinSummary.link(container, ctx);

	container.modal({
		"backdrop":"static"
	});
	container.modal('show');

	onCartSummaryLoaded();
	//for dine in 
	$("#closeDineinSummary").tap(function(){	
		if(typeof closeCartSummary === "function"){
			closeCartSummary();
		} else {
			container.modal("hide");
			$(".container-fluid").css("overflow-y", "unset");
			$(window).scrollTop(scrollposition);
		}
	});
	readyOrderTapped();
}

//common
function addTopMerginForMenu(){
	$("#container").css("margin-top","180px");
}

function viewCartClicked()
{
	if($("#viewCartBtn").hasClass("cartButton-contianer")){
		sortAndMergeMenuItems(context.cart);
		scrollposition = $(window).scrollTop() || $("body").scrollTop();  //save scroll position
		$(".container-fluid").css("overflow-y","hidden");  //scroll to 0
		$(".container-fluid").scrollTop(scrollposition);
		
		if (window.isSTO && !window.disableSharedCart) getSharedCart();
		else showCartSummary();
		if (window.afterViewCartClicked) {
			window.afterViewCartClicked();
		}
	} 
}


//******** go under home.base.js
function showDiv(n){ 
	var x = n;
	var dots = document.getElementsByClassName("demo");
  if (x == 1) {
  	$(".jumbotron-restaurantAds").css('background-image','url(/images/foodbg_1.jpg)');
  } else if (x == 2){
  	$(".jumbotron-restaurantAds").css('background-image','url(/images/asianfood.jpg)');
  } else if (x == 3){ 
	$(".jumbotron-restaurantAds").css('background-image','url(/images/placeholder-restaurants.jpg)');
	}
}


//given results hashmap, append results by each ctgy to the searchlay_container
function showSearchResult(results){
	for(var cate in results)
	{
		var results_category_div = $('<div class="S-category"></div>');
		var results_category_name_div = $("<div class='S-category-name-container'></div>");
		var results_category_name = $("<div class='S-category-name'></div>");
		var span = $('<span>').text(cate);
		$(results_category_name).append(span);
		$(results_category_name_div).append(results_category_name);
		$(results_category_div).append(results_category_name_div);

		results[cate].forEach(function(e)
		{
			appendToSearchlay(e, results_category_div);
		})
		$(".searchlay_container").append(results_category_div);
	}
}

//refactored code from showSearchResults
function appendToSearchlay(e, results_category_div){
	var results_dishInfo = $("<div class='S-dishInfo'></div>");	
	var results_dishName = $("<div class='S-dishName'></div>");
	$(results_dishName).append($('<span>').text(e.name));
	if(e.subtitle != ""){
		$(results_dishName).append($('<p>').text(e.subtitle));
	}
	var results_dishPrice = $("<div class='S-dishPrice font-SSP'></div>");
	$(results_dishPrice).append($('<span>').text(e.price));

	$(results_dishInfo).append(results_dishName);
	$(results_dishInfo).append(results_dishPrice);
	$(results_category_div).append(results_dishInfo);
}

function getSearchResults(){
	var results = {};
	var input_name = $("#search_input").val().toLowerCase().replace(/ /g, '');
	if(input_name === '')
		return;
	var items = $('.listviewOverlay.menuitem-Container');
	var isList = true;
	if(items.length === 0)
	{
		items = $('.dishInfo-Container');
		isList = false;
	}
	function getInitials(str) {
		if (!str) return '';
		var initials = '';
		// English word initials
		initials += str.split(' ').map(function(w){ return w.charAt(0); }).join('');
		// Pinyin initials
		if (typeof Pinyin !== 'undefined') {
			try {
				var py = Pinyin.convertToPinyin(str, ' ', true);
				initials += py.split(' ').map(function(w){ return w.charAt(0); }).join('');
			} catch(e){}
		}
		return initials.toLowerCase();
	}
	function checkIfinputInMenus(dishName,input_name,results,category,price,subtitle)
	{
		var nameLC = dishName.toLowerCase().replace(/ /g, '');
		var matched = nameLC.indexOf(cht2chs(input_name))>=0
			|| nameLC.indexOf(chs2cht(input_name))>=0
			|| getInitials(dishName).indexOf(input_name) >= 0;
		if(matched)
		{
			if(typeof results[category] === 'undefined')
				results[category] = [];
			var dishInfo = {
				'name': dishName,
				'price': price,
				'subtitle': subtitle,
			};
			results[category].push(dishInfo);
		}
	}
	var subtitle = "";
	if(isList){
		items.each(function(){			
			var curId = "#" + $(this).attr("id");
			var dishName = $(curId).find('.dishNameHolder')[0].innerText;
			var price = $(curId).find(".price_btn span")[0].innerText;
			var category = $(curId).find(".dishInfo-Container").attr("category");
			if($(curId).find(".dishSubtitleHolder").length > 0){
				subtitle = $(curId).find(".dishSubtitleHolder")[0].innerText;
			} else {
				subtitle = "";
			}
			checkIfinputInMenus(dishName,input_name,results,category,price,subtitle);
		});
	}else
	{
		items.each(function(){		
			var dishName = $(this).children()[0].innerText;
			var price = $(this).children()[1].innerText;
			var category = $(this).attr("category");
			var subtitle = $(this).attr("subtitle");
			checkIfinputInMenus(dishName,input_name,results,category,price,subtitle);
		});
	}
	return results;
}
	

//only for all-code & dinein
function searchBtnTapped(){
	$('.search_input_out').attr('class','search_input');
	$(".search_input").show();
	$('.searchlay_container').empty();
	//clear inputbox
	$("#search_input").val('');
	setTimeout(function ()
	{
		$("#search_input").select();
		$("#search_input").focus();
	}, 400);
	
	// show search layout and hide dishes menu
	beforeShowSearchResult();
}

//only for mesh & dinein
function dishInfoTapped(obj){
	var currentTime = (new Date()).getTime();
	if (currentTime - window.timeLastScroll < 500)
	{
		return false;
	}	
	categories_clicked = true;
	removefilter();
	//remove_highlightbar();
	var input_name = $(obj).find('.S-dishName span')[0].innerText;
	var input_price = $(obj).find('.S-dishPrice span')[0].innerText;
	var items = $('.listviewOverlay.menuitem-Container');
	var isList = true;
	if(items.length === 0)
	{
		items = $('.menuitem-Container');
		isList = false;
	}
	function enableDisableHeaderMove(dishName, input_name, item, dishPrice, input_price)
	{
		if (dishName.replace(/\s/g, "") === input_name.replace(/\s/g, "") && dishPrice.replace(/\s/g, "") === input_price.replace(/\s/g, ""))
		{
			disableHeaderMove = saveScrollandloadDetails(item, disableHeaderMove);
			return false;
		}
	}
	if(isList){
		items.each(function(){			
			var curId = "#" + $(this).attr("id");
			var dishName = $(curId).find('.dishInfo-Container span')[0].innerText;
			var dishPrice = $(curId).find('.price_btn span')[0].innerText;
			return enableDisableHeaderMove(dishName, input_name, this, dishPrice, input_price);
		});
	}else
	{
		items.each(function(){			
			var dishName = $(this).children().last().children().first().children()[0]['innerText'];
			var dishPrice = $(this).find('.dishPriceHolder span')[0].innerText;
			return enableDisableHeaderMove(dishName,input_name,this, dishPrice, input_price);
		});
	}
}

function searchInputTapped(){
	$('.waterfall').hide();
	$(".searchlay_container").empty();	
		
	var results = getSearchResults();	
	//show searchlay
	showSearchResult(results);
	
	if(JSON.stringify(results) === "{}"){
		var noResultMsg = $(`<div class="S-category"><div class="S-dishInfo"><div class="S-dishName"><span>No Result</span></div></div></div>`);
		$(".searchlay_container").append(noResultMsg);
	}
}

//for mesh & dinein
function scrollCtgyHeader(obj, categoriesWidth, l_this_attr_name){
	var left = 0;
	if(l_this_attr_name === 'All')
	{
		left = 0;
		apply_highlightbar($('#scrollbar-nav [category="'+$("#scrollbar-nav").children().first().attr('category')+'"]'));
	}else{
		left = categoriesWidth[l_this_attr_name][1]-$(window).width()/2-categoriesWidth[l_this_attr_name][0]/2;
		apply_highlightbar($('#scrollbar-nav [category="'+obj.attributes["category"].value+'"]'));
	}
	$('.category-scroll').scrollLeft(left);
}

//for mesh & dinein
function scrollItemList(l_this_attr_name){
	$(".waterfall").show();
	var scrollWindows = 0;
	var categories = $('.S-category-name');
	function scrollToDishFromFilter(itemTop,category,l_this_attr_name,offset)
	{
		if(l_this_attr_name.replace(/\s/g, "") === category.replace(/\s/g, "")){
			itemTop = (itemTop - 111) + offset;
			scrollWindows = itemTop;
			return false;
		}else
		if(l_this_attr_name === 'All')
		{
			scrollWindows = 60;
			return false;
		}
	}
	categories.each(function(){		
		var itemTop = $(this).offset().top;
		var category = $(this).find('span')[0].innerText;
		return scrollToDishFromFilter(itemTop,category,l_this_attr_name,0);
	});

	$(window).scrollTop(scrollWindows);
}

function twoLevelCtgyTapped(obj, categoriesWidth){
	categories_clicked = true;
	onLevelTwoCategory = false;
	scrollLevTwoPageSaver = $(window).scrollTop();
	//removefilter();
	remove_highlightbar();
	$(obj).addClass('twoLevel-ctgyname-tapped');
	$('.navbar-default').addClass('shorterHeaderHeight');
	// waiting #ms then show dishes
	setTimeoutScroll(obj, categoriesWidth);
}

// waiting #ms then show dishes
function setTimeoutScroll(obj, categoriesWidth){
	setTimeout(() => {
		var l_this_attr_name = obj.attributes["category"].value;
		showElemsAfterShowTwoLevelCategory();
		scrollCtgyHeader(obj, categoriesWidth, l_this_attr_name);
		scrollItemList(l_this_attr_name);

		//show switch after tap category
		$('.header-banner').addClass('header-down-nomove');
		$(window).scrollTop($(window).scrollTop() - 58);
		$(obj).removeClass('twoLevel-ctgyname-tapped');

		// hack for shitty Safari...
		$(".waterfall").css("height", 30);
		setTimeout(function(){
			$(".waterfall").css("height", "");
		}, 50);
	}, 300);
}

function showMenuPageHeader(context){
	var menuHeaderRestName = $("#menuHeaderRestName");
	$.templates.menuheaderrest.link(menuHeaderRestName, context);
	$("#menuHeaderRestName").css("display","block");
	$(".backBtnContainer").tap(function(){	
		//close search
		if(onSearchPage) {
			onSearchPage = false;
			// hide search layout and show dishes menu
			afterShowSearchResult();
			$(window).scrollTop(scrollSearchPageSaver);
		}

		onLevelTwoCategory = true;
		hideElemsBeforeShowTwoLevelCategory();		
		$('.header-banner').removeClass('collapsed');
		$('.navbar-default').removeClass('shorterHeaderHeight');
		$(window).scrollTop(0);
		$('.twoLevel-ul').show();
		$('.twoLv-hiddenHeader').show();
		context.enableDualLang = window.enableDualLang;
		$.templates.ordertogohomeheader.link(menuHeaderRestName, context);		
		onMeshHeaderLoaded();
	});
}
