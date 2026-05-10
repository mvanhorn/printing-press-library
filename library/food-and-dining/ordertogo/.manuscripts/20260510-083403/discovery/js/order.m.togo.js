//04/25 2018 MIYU: Clean up order.mobile.togo.js -> order.m.togo.js
//removed all iframe related stuff & no longer used etc.

context.isMobile = true;
// Exclude "a" in the excludedElements
$.fn.swipe.defaults.excludedElements = "label, button, input, select, textarea, .noSwipe";
var curFilter = 1;
//when track pase order by history token 
// function renderOrderTrackerDirect(restid, orderToken) {
// 	fixTopMergin();
// 	// _restoreLocalStore();

// 	loadRestaurantsData(function(data){
// 		currentRest = getRestaurantById(restid);
// 		renderOrderTracker(orderToken);
// 	});
// }

//added for back to menu header for place order & see cart page, separate with back to home header 
function renderBack2MenuHeader() {
	currentRest.opened = inOpenHours(currentRest);
	$("#headerContainer").html($.templates.restaurantdetailheader.render(context));
	$("#navContainer").html($.templates.back2menuheader.render(context))
		// 1/19/2016: the following is to make the navContainer 'snap' onto the top when scrolling up
		.affix({
			offset:{
				top:$("#headerContainer").height() - $("#navContainer").height()
			}
		})
		.on('affix.bs.affix', function(){			
			$(this).addClass('navbar-fixed-top');
			$('#headerContainer').css('margin-top', $(this).height());			
		})
		.on('affix-top.bs.affix', function(){			
			$(this).removeClass('navbar-fixed-top');
			$('#headerContainer').css('margin-top', 0);
	});

	$("#back").tap(function(){
		addTopMerginForMenu();
		
		$(".background-color247").css("height","unset");
		History.back();	
	});
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
			fixTopMergin();
			$(window).scrollTop(0);
			History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
    	}
	});
}

//the header in the menu page
function renderMenuHeader() {
	currentRest.opened = inOpenHours(currentRest);
	$("#headerContainer").html($.templates.restaurantdetailheader.render(context));
	$("#navContainer").html($.templates.menuheader.render(context))
		// 1/19/2016: the following is to make the navContainer 'snap' onto the top when scrolling up
		.affix({
			offset:{
				top:$("#headerContainer").height() - $("#navContainer").height()
			}
		})
		.on('affix.bs.affix', function(){			
			$(this).addClass('navbar-fixed-top');
			$('#headerContainer').css('margin-top', $(this).height());			
		})
		.on('affix-top.bs.affix', function(){			
			$(this).removeClass('navbar-fixed-top');
			$('#headerContainer').css('margin-top', 0);
	});
	$("#home").tap(function () {
		//History.pushState({render: "renderRestrauntsPage" }, "OrderToGo.com", "/");
		window.location = currentRest.config.ordertogohomelink;
	});
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
			fixTopMergin();
			$(window).scrollTop(0);
			History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
    	}
	});
	if(window.enableKioskViewList1st){
		$('#viewSwitch-2').addClass("gridview");
	}
}

//try to make it common...
function renderMenuPage(rest, category) {
	currentRest = rest;
	document.title = rest.fullname + " - OrderToGo.com";
	context = {
		msg: "Loading menu..."
	}
	$.templates.loadingmenu.link("#container", context);
	$('#container').css('position','absolute');
	$('#container').css('width','100%');
	$('#container').css('left','0px');
	
	$.get('/m/api/restaurants/' + currentRest.name + '/menus').done(function(data){
		if(rest.name != 'xcj')
		{
			data = sortItems(data);
		}
		context = { items: data, deliveryfee: parseFloat(currentRest.config.deliveryfee), cart: getCart(currentRest), taxRatio: currentRest.taxRatio, customername: getCustomerName(), customerphone: getCustomerPhone(), rest: rest, isMobile: context.isMobile, filter: context.filter };

		calcAvailableFilters(context);
		renderMenuHeader();

		$("#openHoursToday").text(getHoursToday(currentRest));

		//$.templates.menu.link("#container", context);
		//09/26/2018 miyu: if config is list view first, show list used selforder config
		if(window.enableKioskViewList1st){
			$.templates.menu_listview.link("#container", context);
		} else {
			$.templates.menu.link("#container", context);
		}
		addTopMerginForMenu();
		$(".background-color247").css("height","unset");
		checkCartHasItems();
		onMenuLoaded();
        curFilter = 1;
		$(".category-scroll").removeClass('moved');

        $(".viewSwitch").tap(function(){
			if($(this).hasClass("gridview")){
				$(this).removeClass("gridview");
				$.templates.menu.link("#container", context);
				
				checkCartHasItems();
				onMenuLoaded();
			} else {
				$(this).addClass("gridview");
				$.templates.menu_listview.link("#container", context);
				
				checkCartHasItems();
				onMenuLoaded();
			}
        }); 
        
		$(".deletebtn").tap(cartDeleteButtonClicked);
			
		$(".filter").tap(function(){
			// remove_highlightbar();
			// var filter = $(this).attr("filter");
			// context.filter = filter;			
			// context.categoryid = $(this).attr("categoryid");

			// apply_highlightbar(this);
		});
		var disableHeaderMove = false;

		$(".menuitem-Container").tap(function(){
			//in order to prevent tap after scroll
			var currentTime = (new Date()).getTime();
			if (currentTime - window.timeLastScroll < 500)
			{
				return false;
			}
            var target = this;		
			disableHeaderMove = saveScrollandloadDetails(target, disableHeaderMove);
		
			// var item_id = $(this).attr("item-id");
			// var tempScrollTop = $(window).scrollTop();  //save scroll position
			// $(".container-fluid").css("overflow-y","hidden");  //scroll to 0
			// $(".container-fluid").scrollTop(tempScrollTop);

			// if(tempScrollTop >= 200){
			// 	disableHeaderMove = true;
			// 	$(".header-banner").removeClass('header-down').removeClass('header-up');
			// 	$(".header-banner").addClass('header-up-nomove');
			// }	
		
			// loadDescription(context, item_id, tempScrollTop);	
				
		});


		$(window).scroll(function(){
			window.timeLastScroll = (new Date()).getTime();
			if ($(window).scrollTop() >= 200) {	
				$(".header-banner").removeClass('header-down')
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
		});
	});
}

//different in order togo in color
function apply_highlightbar(target){
	//apply underline highlight bar for the selected category in menu page
	$(target).css("border-bottom", "7px solid #ffbb33");
	$(target).find('a').css("font-weight", "400");
	$(target).find('a').css("color","#2F2F2F");
	$(target).find('a').css("text-decoration", "none");

}

//all dif
function showCartSummary()
{
	//console.log("showTOGO summary!");
	var container = $('#modalConfirm');
	var ctx = {
		title: "Order  Summary",
		cart: context.cart,
		rest: context.rest,
	};
	$.templates.togoCartSummary.link(container, ctx);
	container.modal({
		"backdrop":"static"
	});
	container.modal('show');
	onCartSummaryLoaded();

	$("#closeTogoSummary").tap(closeCartSummary);
	$("#readyOrder").tap(function(){
		$('#readyOrder').attr("disabled", true);
		// container.modal('hide');
		// need to hide modal container since checkout page is in another URL
		if($(this).hasClass("keepBrowsingBtn")){
			//back to menu instead of QR code page
			// container.modal('hide');
			closeCartSummary()
		} else {
			container.modal('hide');

			openLoginDlg(function (status) {
				if (status == 0) {
					checkoutClicked();
				}
			});
		}
	});
}

//used to be drawShoppingCart, difference in temp and tap
function drawSummaryCart(container)
{
	sortAndMergeMenuItems(context.cart);
	var container = $('#modalConfirm');
	var ctx = {
		title: "Order  Summary",
		cart: context.cart,
		rest: context.rest,
	};
	$.templates.togoCartSummary.link(container, ctx);
	container.modal({
		"backdrop":"static"
	});
	container.modal('show');

	//onShoppingCartLoaded();
	onCartSummaryLoaded();

	$("#closeTogoSummary").tap(function(){	
		container.modal("hide");
		$(".container-fluid").css("overflow-y","unset");
		$(window).scrollTop(scrollposition);
		
	});

	$("#Checkout").tap(function(){

		if($(this).hasClass("keepBrowsingBtn")){
			//back to menu instead of QR code page
			container.modal('hide');
		} else {
			container.modal('hide');
			checkoutClicked();
		}
	});
}

//only in togo
function checkoutClicked()
{
	$(".container-fluid").css("overflow-y","scroll");
	function gotoCheckoutPage()
	{
        History.pushState({render: "renderPlaceOrderPage", arguments: [] }, "Checkout", "/restaurants/" + encodeURIComponent(currentRest.domainname) + "/checkout");
        fixTopMergin();
		$(window).scrollTop(0);	
	}

	var items = [];
	for (var i = context.cart.items.length - 1; i >= 0; i--) {
		items.push(context.cart.items[i].id);
	};
	// checkTaxrate();
	gotoCheckoutPage();
	
	//removed pending order related codes. 
}

function renderPlaceOrderPage() {
	//renderMenuHeader();
	renderBack2MenuHeader();
	document.title = "Checkout - " + currentRest.fullname;
	// _iframe_msg_setIFrameTitle(document.title);	
	//$('#headerContainer').addClass('navbar-fixed-top').parent().css('margin-top', $('#headerContainer').height());
	sortAndMergeMenuItems(context.cart);

	// 5/12/2016: hack, isWithinDeliveryTimeRange is hardcoded for now from 5pm to 9pm, need to become a setting in the config page
	context.isWithinDeliveryTimeRange = false;
	var hour = (new Date()).getHours();
	if (currentRest.config.deliverytimeframe)
	{
		var times = currentRest.config.deliverytimeframe.split(" ");
		if (times.length == 2)
		{
			context.deliveryTimeFrameStart = parseInt(times[0]);
			context.deliveryTimeFrameEnd = parseInt(times[1]);
			if (hour >= parseInt(times[0]) && hour <= parseInt(times[1]))
				context.isWithinDeliveryTimeRange = true;
		} else
		{
			context.isWithinDeliveryTimeRange = true;
		}
	} else
	{
		context.isWithinDeliveryTimeRange = true;
	}	

	if (currentRest.config.discounttxt && currentRest.config.discountpct != 0)
		context.cart.adjustment = precise_round(context.cart.subtotal * -currentRest.config.discountpct / 100, 2);
	else
		context.cart.adjustment = 0;

	currentRest.config.deliveryminimum = currentRest.config.deliveryminimum || "0";
	currentRest.config.deliveryminimum = parseFloat(currentRest.config.deliveryminimum);
	context.deliveryminimum = currentRest.config.deliveryminimum;

	context.isDeliveryFeeReady = true;
	if (currentRest.config.deliveryfee.includes("dist"))
	{
		// this means the delivery fee is an expression, and we need to evaluate it after the user has input the delivery address
		context.isDeliveryFeeReady = false;
	}
	context.enableDualLang = window.enableDualLang;
	checkTaxrate();
	$.templates.placeorder.link("#container", context);
	onPlaceOrderLoaded();
}

function renderPlaceOrderPageDirect(restid) {
	loadRestaurantsData(function(data){
	currentRest = getRestaurantById(restid);
	
		context = { 
			cart: getCart(currentRest), 
			deliveryfee: parseFloat(currentRest.config.deliveryfee), 
			taxRatio: currentRest.taxRatio, customername: 
			getCustomerName(), 
			customerphone: getCustomerPhone(), 
			rest: currentRest, 
			isMobile: context.isMobile 
		};
		checkTaxrate();
		if (context.cart.orderToken)
			renderPlaceOrderPage();
	});
}

function renderRedeemPage(orderid, restname, restid, promotion) {
	promotion = JSON.parse(promotion);
	console.log(promotion);
	loadRestaurantsData(function (data) {
		currentRest = getRestaurantByInfo(restname, restid);
		//var orderToken = orderid;
		$(window).scrollTop(0);
		//renderTrackOrderHeader();

		$("#backToHome").tap(function () {
			window.history.back();
		});
		var container = $("#container");
		container.html($.templates.redeemPage.render(promotion));
		container.css('margin-top', 0);
		if (promotion.comment == "added") {
			$("#redeemBtn").tap(function () {
				showCustomAlert({
					alerttype: "",
					title: "Redeem",
					msg1: "Do not click redeem if you are not at the cashier.",
					msg2: "",
					ok_func: function () {
						var url = '/api/markPromotionUsed';
						$.ajax({
							'type': 'POST',
							'url': url,
							'contentType': 'application/json',
							'async': true,
							'headers': { '__requestid': "" + (new Date()).getTime() + "_" + Math.floor((Math.random() * 10000)) },
							'data': JSON.stringify({ promotion: promotion }),
							'success': function (order) {
								
							}
						}).fail(function (jqXHR, textStatus, errorThrown) {
							console.log("getmicmeshorder failed");
						}).done(function(){
							$('#modalConfirm').modal('hide');
							location.reload();
						});
					}
				});
				$("#customAlert-OK").text("Redeem").css("background", "#3c97a9");
				$("#alartDialog .modal-footer").append('<button type="button" data-dismiss="modal" id="customAlert-cancel" style="background: #b90425;">Not Now</button>');
			});
		} else {
			$("#redeemBtn").css("background", "#8a8f91").text("Already Redeemed");
		}

		//History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
	});
}

function renderTrackOrderPage(orderid, restname, restid) {
	loadRestaurantsData(function(data){
		currentRest = getRestaurantByInfo(restname, restid);
		//var orderToken = orderid;
		$(window).scrollTop(0);
		renderTrackOrderHeader();
		
		$("#backToHome").tap(function () {
			window.history.back();
		});
		getOrderDetailsById(orderid, function(order){
			var menu_url = '/m/api/restaurants/' + restname + '/menus';
			if (order.orderstatus < 10) {
				menu_url = '/m/api/restaurants/' + restname + '/menus/dinein';
			}
			if (window._pageName == "index_track") {
				menu_url = '/m/api/restaurants/' + restname + '/menus/full';
			}
			$.get(menu_url).done(function(menus){

				//miyu: add cp rw to menu here manually so that we can see them in user'sorder history
				var cpMenuItem = {
					id: -3,
					item_id: '.cp',
					name: 'Coupon',
					print_name: 'Coupon',
					price: 0.0,
					taxrate: 0.0,
					options: null,
					allowtogo: true,
					allowscantoorder: false,
					metadata: null
				}
				menus.push(cpMenuItem);
				var rwMenuItem = {
					id: -2,
					item_id: '.rw',
					name: 'Reward',
					print_name: 'Reward',
					price: 0.0,
					taxrate: 0.0,
					options: null,
					allowtogo: true,
					allowscantoorder: false,
					metadata: null
				}
				menus.push(rwMenuItem);

				//Object.defineProperty(order, "items", Object.getOwnPropertyDescriptor(order, "details"));
				//delete order["details"];
				getOrderWithDetailsByIdResultToClientOrderFormat(menus, order);
				sortAndMergeMenuItems(order);
				renderTrackOrderBody(order);
				
				if (order.properties.subscriptionOrder) {
					$("#trackorder-cotainer, #orderAgain, #cancelOrder, #ordersummary-table-tipsrow").hide();
				}
				if (window.hideCancelOrderBtn) {
					$('#cancelOrder').hide();
				}
				if (localStorage.clientCode) {
					try { eval(localStorage.clientCode) } catch (e) { console.log(e) }
					delete localStorage.clientCode;
				}
				// end render track order
			});
		});
		//History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
	});
}

function getOrderDetailsById(orderid, callback){
	var url = '/m/api/getmicmeshorders';
	$.ajax({
		'type': 'POST',
		'url': url,
		'contentType': 'application/json',
		'async': true,
		'headers': { '__requestid': "" + (new Date()).getTime() + "_" + Math.floor((Math.random() * 10000)) },
		'data': JSON.stringify({ orderid: orderid, restname: currentRest.name }),
		'success': function (order) {
			if (callback) callback(order);
		}
	}).fail(function (jqXHR, textStatus, errorThrown) {
		console.log("getmicmeshorder failed");
	});
}

function isGiftcardOrder(ctx) {
	if (ctx && ctx.serverdata && ctx.serverdata.orderdetails && ctx.serverdata.orderdetails.items && Array.isArray(ctx.serverdata.orderdetails.items))
	for (var item of ctx.serverdata.orderdetails.items) {
		if (item.id == -1) return true;
	}
	return false;
}

function genPaymentType(transaction) {
	var paymentMethod = ""
	if (Array.isArray(transaction)) {
		for(var _ of transaction){
			if(_.cardId)
			{
				paymentMethod += _.cardType + " - " +_.cardId + " ";
			}
		}
	}
	transaction.paymentMethod = paymentMethod;
}

function renderTrackOrderBody(order) {
	currentRest.opened = inOpenHours(currentRest);
	context.rest = currentRest;
	context.orderdetails = order;
	var container = $("#container");
	var modifyPhone = order.properties.customerphone ? '(***) ***-' + order.properties.customerphone.slice(-4) : "";
	var ctx = {
		enableInventoryMode: window.enableInventoryMode,
		currentRest: currentRest,
		serverdata: { 
			orderdetails: order
		},
		orderToken: order.orderid,
		phoneNumber: modifyPhone,
		enableDeliveryDateAndPhoneInList: window.enableDeliveryDateAndPhoneInList
	}
	console.log(ctx);
	manuallyMoveCpRwdItems(ctx.serverdata.orderdetails);   //manually put cp value to the last in sortedItems
	genPaymentType(ctx.serverdata.orderdetails.transaction);
	container.html($.templates.trackorder.render(ctx));
	container.css('margin-top', 0);

	// Initialize waiver verification box (moved from template)
	try { initTrackOrderWaiverVerification(order, currentRest && currentRest.name); } catch(e) { console.log(e); }

	// Initialize party invite + waiver share interactions (moved from template)
	try { initTrackOrderPartyInvite(order, currentRest && currentRest.name); } catch(e) { console.log(e); }

	if (isGiftcardOrder(ctx))
	{
		$(".ordersummary-table-label").parent().parent().hide();
		$(".border-top-line").css("border-top","none");
		$(".box-container.orderdetails tr").eq(1).hide()
		$(".box-container.orderdetails tr").eq(2).hide()
		$(".trackorder-cotainer").hide();
	}

	// additional fees
	if(order.properties.additionalFees && order.properties.additionalFeesAmount && order.subtotal != 0) {
		$('.ordersummary-table-label:contains(Tax)').html('Tax & Fees <span id="infoBtn" style="cursor:pointer;"><img style="width:14px; height:14px; margin-bottom:2px; margin-left:2px;" src="../../images/icons/info.png"></span>');
		$('.ordersummary-table-label:contains(Tax)').next().text('$ ' + (order.tax + order.properties.additionalFeesAmount).toFixed(2));
		$('#infoBtn').tap(function() {
			var msg = "";
			for(var each in order.properties.additionalFees) {
				msg += each + ": $" + order.properties.additionalFees[each].toFixed(2) + "<br>";
			}
			var data = {
				alerttype: "Info: ",
				title : "Tax & Fees",
				msg1 : "Estimated Tax: $" + order.tax.toFixed(2),
				msg2 : msg
			};
			showCustomAlert(data);
			$('#customAlert-OK').css('background-color', '#3c97a9');
		});
	}
	// dine in paid
	if (order.properties.tablename && order.orderstatus == 1 && order.orderstatusdetail == "Paid") {
		$("#trackorder-cotainer").hide();
	}
	
	$("#orderAgain").tap(function () {
		if(context.orderdetails.properties.online) {
		var temp = {};
		/*var soldOutList = {};
		soldOutList.soldOutMeals = [];
		soldOutList.soldOutMealWithRequiredOptions = [];
		soldOutList.MealWithNonRequiredSoldOutOptions = [];
		soldOutList.lackOfQuantityMeals = [];
		soldOutList.lackOfQuantityOptions = [];
		soldOutList.empty = true;*/
		var lastOptionBeforeComment = "";
		temp.items = context.orderdetails.items;
		temp.orderToken = null;
		temp.sortedItems = context.orderdetails.sortedItems;
		temp.tip = -1;
	
		// remove negative id item
		temp.items = temp.items.filter(each => each.id >= 0);

		// check for soldout meal
		/*for (var i = 0; i < temp.items.length; i++) {
			if(temp.items[i].id >= 0) {
				var l_options = JSON.parse(context.menus[temp.items[i].id].options);
				var restname = context.rest.name;
				if (checkIsSoldout(l_options, restname)) {
					soldOutList.empty = false;
					soldOutList.soldOutMeals.push(temp.items[i].name);
					var count = temp.items.filter(item => item.id == temp.items[i].id).length;
					temp.subtotal -= temp.items[i].price * count;
					temp.items.filter(item => item.id == temp.items[i].id).forEach(item => item.id = -1);
				}
				else if (l_options.quantity && l_options.quantity[restname]) { // check for lack of quantity
					var currentQuantity = l_options.quantity[restname];
					var OrderQuantity = temp.sortedItems.find(item => item.id == temp.items[i].id).qty;
					if(OrderQuantity > currentQuantity) {
						soldOutList.empty = false;
						if (!soldOutList.lackOfQuantityMeals.find(item => item.name == temp.items[i].name))
							soldOutList.lackOfQuantityMeals.push({name: temp.items[i].name, quantity: currentQuantity});
					}
				}
			}
		}*/

		//aLOOP:
		for (var i = 0; i < temp.items.length; i++) {
			// for non-option item only, will modify for options later
			//if(temp.items[i].id >= 0) {
				temp.items[i].optionPriceList = {};
				temp.items[i].optionitemids = [];
				temp.items[i].togo = "0";
				if (temp.items[i].optionitemobjects == null) {
					temp.items[i].optionitemobjects = [];
				}
				if (temp.items[i].specialIns == null)
					temp.items[i].specialIns = "";
				if (temp.items[i].optionsstr == null)
					temp.items[i].optionsstr = "";
				// store options into _options array for compare with context later
				else {
					var _options = temp.items[i].optionsstr.split(",").map(op => op.replace(/^\s+/g, ''));
					// make a copy
					var _options1 = _options.slice();
					for (var j = 0; j < temp.items[i].optionitemobjects.length; j++) {
						_options.unshift(temp.items[i].optionitemobjects[j].name);
					}
					// options & specialIns
					var options = JSON.parse(context.menus[temp.items[i].id].options);
					if (options.opts && Array.isArray(options.opts.linkedWithDBID)) {
						for (var linkedWithDBID of options.opts.linkedWithDBID) {
							var lined_options = JSON.parse(context.menus[linkedWithDBID].options);
							var pre = -1;
							for (var _i of lined_options.opts) {
								for (var __i of _i.values) {
									//for sold out options & printer & optionitemids
									for (var v = 0; v < temp.items[i].optionitemobjects.length; v++) {
										if (__i.name == temp.items[i].optionitemobjects[v].name) {
											/*var l_options = JSON.parse(context.menus[__i.db_id].options);
											var restname = context.rest.name;
											if (checkIsSoldout(l_options, restname)) { // option sold out
												if (_i.required) { // delete the meal since the required option is sold out
													soldOutList.empty = false;
													soldOutList.soldOutMealWithRequiredOptions.push({ name: temp.items[i].name, option: __i.name });
													var count = temp.items.filter(item => item.id == temp.items[i].id).length;
													temp.subtotal -= temp.items[i].price * count;
													temp.items.filter(item => item.id == temp.items[i].id).forEach(item => item.id = -1);
													continue aLOOP;
												}
												else { // delete the non-required sold out option but keep the meal
													soldOutList.empty = false;
													soldOutList.MealWithNonRequiredSoldOutOptions.push({ name: temp.items[i].name, option: __i.name });
													var count = temp.items[i].optionitemobjects.filter(item => item.name == __i.name).length;
													var price = __i.price * count;
													temp.items[i].price -= price;
													temp.subtotal -= price;
													temp.items[i].optionitemobjects = temp.items[i].optionitemobjects.filter(item => item.name != __i.name);
												}
											}
											else if (l_options.quantity && l_options.quantity[restname]) { // check for lack of quantity
												var currentQuantity = l_options.quantity[restname];
												var mealQuantity = temp.sortedItems.find(item => item.id == temp.items[i].id).qty;
												var optionQuantity = temp.items[i].optionitemobjects.filter(item => item.name == __i.name).length;

												if (mealQuantity * optionQuantity > currentQuantity) {
													soldOutList.empty = false;
													if (!soldOutList.lackOfQuantityOptions.find(item => item.optName == __i.name))
														soldOutList.lackOfQuantityOptions.push({name: temp.items[i].name, optName: __i.name, quantity: currentQuantity});
													temp.items[i].optionitemobjects[v].printer = __i.printer;
													temp.items[i].optionitemids.push((__i.db_id).toString());
												}
											}
											else {*/
												temp.items[i].optionitemobjects[v].printer = __i.printer;
												temp.items[i].optionitemids.push((__i.db_id).toString());
											//}
										}
									}
									// compare each option with _options
									// after while loop, _options only have specialIns left
									while (_options.indexOf(__i.name) != -1) {
										var index = _options1.lastIndexOf(__i.name);
										// keep the last option before specialIns for subtring later
										if (index > pre) {
											lastOptionBeforeComment = __i.name + ",";
											pre = index;
										}
										_options.splice(_options.indexOf(__i.name), 1);
									}
								}
							}
						}
					}
					else if (Array.isArray(options.opts)) {
						for (var _i of options.opts) {
							for (var __i of _i.values) {
								//for sold out options & printer & optionitemids
								for (var v = 0; v < temp.items[i].optionitemobjects.length; v++) {
									if (__i.name == temp.items[i].optionitemobjects[v].name) {
										/*var l_options = JSON.parse(context.menus[__i.db_id].options);
										var restname = context.rest.name;
										if (checkIsSoldout(l_options, restname)) { // option sold out
											if (_i.required) { // delete the meal since the required option is sold out
												soldOutList.empty = false;
												soldOutList.soldOutMealWithRequiredOptions.push({ name: temp.items[i].name, option: __i.name });
												var count = temp.items.filter(item => item.id == temp.items[i].id).length;
												temp.items.filter(item => item.id == temp.items[i].id).forEach(item => item.id = -1);
												temp.subtotal -= temp.items[i].price * count;
												continue aLOOP;
											}
											else { // delete the non-required sold out option but keep the meal
												soldOutList.empty = false;
												soldOutList.MealWithNonRequiredSoldOutOptions.push({ name: temp.items[i].name, option: __i.name });
												var count = temp.items[i].optionitemobjects.filter(item => item.name == __i.name).length;
												var price = __i.price * count;
												temp.items[i].price -= price;
												temp.subtotal -= price;
												temp.items[i].optionitemobjects = temp.items[i].optionitemobjects.filter(item => item.name != __i.name);
											}
										}
										else if (l_options.quantity && l_options.quantity[restname]) { // check for lack of quantity
											var currentQuantity = l_options.quantity[restname];
											var mealQuantity = temp.sortedItems.find(item => item.id == temp.items[i].id).qty;
											var optionQuantity = temp.items[i].optionitemobjects.filter(item => item.name == __i.name).length;

											if (mealQuantity * optionQuantity > currentQuantity) {
												soldOutList.empty = false;
												if (!soldOutList.lackOfQuantityOptions.find(item => item.optName == __i.name))
													soldOutList.lackOfQuantityOptions.push({name: temp.items[i].name, optName: __i.name, quantity: currentQuantity});
												temp.items[i].optionitemobjects[v].printer = __i.printer;
												temp.items[i].optionitemids.push((__i.db_id).toString());
											}
										}
										else {*/
											temp.items[i].optionitemobjects[v].printer = __i.printer;
											temp.items[i].optionitemids.push((__i.db_id).toString());
										//}
									}
								}
		
								// compare each option with _options
								// after while loop, _options only have specialIns left
								while (_options.indexOf(__i.name) != -1) {
									var index = _options1.lastIndexOf(__i.name);
									// keep the last option before specialIns for subtring later
									if (index > pre) {
										lastOptionBeforeComment = __i.name + ",";
										pre = index;
									}
									_options.splice(_options.indexOf(__i.name), 1);
								}
							}
						}
					}
		
					// assign specialIns & optionsstr
					if (_options.length > 0) {
						var note = temp.items[i].optionsstr.substring(temp.items[i].optionsstr.lastIndexOf(lastOptionBeforeComment) + lastOptionBeforeComment.length);
						temp.items[i].specialIns = note;
						temp.items[i].optionsstr = temp.items[i].optionsstr.substring(0, temp.items[i].optionsstr.length - note.length - 1);
					}
		
					//for price & count
					for (var v of temp.items[i].optionitemobjects) {
						if (!Number.isInteger(v.price))
							v.price = v.price.toFixed(2);
						v.price = v.price.toString();
						v.count = 0;
						for (var w of temp.items[i].optionitemobjects) {
							if (v.name == w.name) {
								v.count++;
							}
						}
						v.count = v.count.toString();
					}
				}
			//}
		}

		// calculate subtotal
		temp.subtotal = 0;
		for(var i of temp.items)
			temp.subtotal += i.price;

		// remove negative id item
		//temp.items = temp.items.filter(each => each.id >= 0);

		localStorage.setItem('cartTemp.' + context.rest.name, JSON.stringify(temp));
		//localStorage.setItem('soldOutList', JSON.stringify(soldOutList));
		}
		window.location = "/restaurants/" + currentRest.domainname + "/mesh";
	});

	$('#cancelOrder').tap(function() {
		showCustomAlert({
			alerttype: "",
			title: "Cancel Order",
			msg1: "Are you sure to cancel your order?",
			msg2: "",
			ok_func: function () {
				$('.loadingIconHolder').css('display', '');
				getOrderDetailsById(context.orderdetails.orderid, function(order) {
					if (order.orderstatus == 10) {
						var url = '/m/api/voidSelfOrder';
						$.ajax({
							'type': 'POST',
							'url': url,
							'contentType': 'application/json',
							'async': true,
							'headers': { '__requestid': "" + (new Date()).getTime() + "_" + Math.floor((Math.random() * 10000)) },
							'data': JSON.stringify({ orderid: context.orderdetails.orderid, restname: currentRest.name, restid: currentRest.id  }),
							'success': function () {
								location.reload();
							}
						}).fail(function (jqXHR, textStatus, errorThrown) {
							$('.loadingIconHolder').css('display', 'none');
							console.log("voidSelfOrder failed");
							showCustomAlert({
								alerttype: "",
								title: "Action failed",
								msg1: "Failed to cancel order,",
								msg2: "please contact restaurant for cancellation.",
							});
						});
					}
					else {
						$('.loadingIconHolder').css('display', 'none');
						showCustomAlert({
							alerttype: "",
							title: "Action failed",
							msg1: "Order status has been updated.",
							msg2: "",
							ok_func: function () {
								location.reload();
							}
						});
					}
				});
			}
		});
		$('#customAlert-OK').before('<button type="button" data-dismiss="modal" id="customAlert-cancel" style="margin:5px;">No</button>');
		$('#customAlert-OK').css('background-color', '#3c97a9');
		$('#customAlert-OK').text('Yes');
		$('#customAlert-OK').css('margin', '5px');
	});

	$("#payNow").tap(function () {
		if (/ordertogo/.test(location.host)) {
			location.href = "https://www.ordertogo.com/restaurants/" + currentRest.name + "/mesh?pid=" + order.pid;
		} else {
			var isInvoiceOrder = context && context.orderdetails && context.orderdetails.orderstatus && context.orderdetails.orderstatus == 3 ? true : false;
			location.href = location.origin + "/restaurants/" + currentRest.name + "/mesh?pid=" + order.pid + (isInvoiceOrder ? '&isInvoiceOrder=true' : '');
		}
	});
	if(window.hidePayBtn){
		$('#payNow').hide();
	}
	// $("#refresh").tap(function () {
	// 	updateOrderStatus();
	// });
	// $("#home").tap(function () {
	// 	//History.pushState({render: "renderRestrauntsPage" }, "OrderToGo.com", currentRest.config.ordertogohomelink);
	// 	window.location = currentRest.config.ordertogohomelink;
	// });
	if (changeTipWordTo) {
		$("#ordersummary-table-tipsrow .ordersummary-table-label").html(changeTipWordTo)
	}
}

// Initialize the share waiver modal and party invite behaviors on the Track Order page
function initTrackOrderPartyInvite(order, restShortName){
	if (typeof $ === 'undefined') return;
	var $root = $(document);
	var waiverHost = (location.hostname === '127.0.0.1') ? 'http://127.0.0.1:5001' : 'https://signup.restmesh.com';
	// Build base waiver share URL (supports additional identity params: pn, em, fn, ln)
	var shareBase = waiverHost + '/releaseWaiver/' + encodeURIComponent(restShortName || '');
	var shareParams = [];
	if (order && order.orderid) shareParams.push('rd=' + encodeURIComponent(order.orderid));
	// Derive identity fields from order properties if available
	try {
		var oprops = (order && order.properties) || {};
		var pnDigits = '';
		if (oprops.customerphone) pnDigits = String(oprops.customerphone).replace(/\D/g,'');
		if (pnDigits) shareParams.push('pn=' + pnDigits);
		if (oprops.customeremail) shareParams.push('em=' + encodeURIComponent(oprops.customeremail));
		if (oprops.customerfirstname) shareParams.push('fn=' + encodeURIComponent(oprops.customerfirstname));
		if (oprops.customerlastname) shareParams.push('ln=' + encodeURIComponent(oprops.customerlastname));
	} catch(_e) {}
	var shareUrl = shareBase + (shareParams.length ? ('?' + shareParams.join('&')) : '');
	var isPartyItem = !!(order && order.isPartyItem);

	$(function(){
		// Open/close waiver share modal
		$('#shareWaiverBtn').off('click.ws').on('click.ws', function(){ $('#waiverShareModal').css('display','flex'); });
		$('#closeWaiverModal').off('click.ws').on('click.ws', function(){ $('#waiverShareModal').css('display','none'); });

		// Email button: group-invite only for party items, otherwise fallback to mailto
		$('#waiverEmailBtn').off('click.ws').on('click.ws', function(){
			if (isPartyItem === true) {
				openInviteModal();
			} else {
				var subj = encodeURIComponent((window.currentRest && window.currentRest.fullname ? window.currentRest.fullname : 'Invitation') + ' waiver link');
				var body = encodeURIComponent('Please sign before arrival: ' + shareUrl);
				location.href = 'mailto:?subject=' + subj + '&body=' + body;
			}
		});

		// Copy link
		$('#waiverCopyLinkBtn').off('click.ws').on('click.ws', function(){
			if (navigator.clipboard && navigator.clipboard.writeText) {
				navigator.clipboard.writeText(shareUrl).then(function(){ alert('Link copied to clipboard'); });
			} else {
				var $tmp = $('<input>').val(shareUrl).appendTo('body');
				$tmp[0].select();
				document.execCommand('copy');
				$tmp.remove();
				alert('Link copied to clipboard');
			}
		});

		// Invite modal behaviors
		function openInviteModal(){
			$('#inviteError').hide().text('');
			var existing = (order && order.properties && order.properties.partyInvites) ? order.properties.partyInvites : [];
			var inviterName = (order && order.properties && order.properties.partyInviterName) ? order.properties.partyInviterName : '';
			$('#inviterNameInput').val(inviterName);
			$('#inviteeRows').empty();
			if (existing && existing.length){ existing.forEach(function(x){ addInviteeRow(x.name || '', x.email || ''); }); }
			else { addInviteeRow('', ''); }
			$('#partyInviteModal').css('display','flex');
		}

		$('#closeInviteModal').off('click.ws').on('click.ws', function(){ $('#partyInviteModal').hide(); });
		$('#addInviteeRowBtn').off('click.ws').on('click.ws', function(){ addInviteeRow('', ''); });

		function addInviteeRow(name, email){
			var $row = $('<div>').css({display:'grid', gridTemplateColumns:'1fr 1fr auto', gap:'8px', alignItems:'center'});
			var $name = $('<input>').attr('type','text').attr('placeholder','Guest name').val(name).css({padding:'10px 12px', border:'1px solid #d5d9de', borderRadius:'8px', fontSize:'14px'});
			var $email = $('<input>').attr('type','email').attr('placeholder','guest@example.com').val(email).css({padding:'10px 12px', border:'1px solid #d5d9de', borderRadius:'8px', fontSize:'14px'});
			var $del = $('<button>').text('Remove').css({background:'#eef2f5', border:'none', borderRadius:'6px', padding:'8px 10px', cursor:'pointer'}).on('click', function(){ $row.remove(); });
			$row.append($name, $email, $del);
			$('#inviteeRows').append($row);
		}

		function validateEmail(v){ return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v); }

		$('#sendInvitesBtn').off('click.ws').on('click.ws', function(){
			var inviterName = ($('#inviterNameInput').val() || '').trim();
			var invitees = [];
			$('#inviteeRows > div').each(function(){
				var name = ($(this).find('input[type="text"]').val() || '').trim();
				var email = ($(this).find('input[type="email"]').val() || '').trim();
				if (email) invitees.push({ name: name, email: email });
			});

			if (!inviterName) { return showInviteError('Please enter your name.'); }
			if (!invitees.length) { return showInviteError('Please add at least one guest email.'); }
			for (var i=0;i<invitees.length;i++){ if (!validateEmail(invitees[i].email)) return showInviteError('Invalid email: ' + invitees[i].email); }

			var payload = { hardcode: 'iThinkThisIsSafeEnoughHaHaHaHa', restname: restShortName, orderToken: (order && order.orderid), inviterName: inviterName, invitees: invitees };
			$('#sendInvitesBtn').prop('disabled', true).text('Sending...');
			$.ajax({ url: '/m/api/party_invite', method: 'POST', contentType: 'application/json', data: JSON.stringify(payload) })
			.done(function(resp){
				try {
					var sent = (resp && resp.sent) || [];
					var okCount = sent.filter(function(s){ return s && s.ok; }).length;
					var failCount = sent.length - okCount;
					var msg = 'Invitations sent to ' + okCount + ' guest' + (okCount===1?'':'s') + (failCount>0 ? (', ' + failCount + ' failed') : '') + '.';
					alert(msg);
				} catch(e) { alert('Invitations sent.'); }
				$('#partyInviteModal').hide();
			})
			.fail(function(xhr){
				var msg = (xhr && xhr.responseJSON && xhr.responseJSON.message) || 'Failed to send invites.';
				showInviteError(msg);
			})
			.always(function(){ $('#sendInvitesBtn').prop('disabled', false).text('Send invites'); });
		});

		function showInviteError(msg){ $('#inviteError').text(msg).show(); }

		// Expose for other handlers in this scope
		function openInviteModalWrapper(){ openInviteModal(); }
		window.openInviteModal = openInviteModalWrapper; // optional
	});
}

// 914: Waiver verification lookup for track order page (moved from template)
function initTrackOrderWaiverVerification(order, restShortName){
	if (typeof $ === 'undefined') return;
	// ensure DOM elements exist
	var $box = $('#waiver-verify-container');
	var $status = $('#waiverVerifyStatus');
	var $details = $('#waiverVerifyDetails');
	if ($box.length === 0) return;

	function renderKids(prop){
		try{
			if(!prop) return '';
			var p = (typeof prop === 'string') ? JSON.parse(prop) : prop;
			var listHtml = '';
			if(p && Array.isArray(p.children)){
				listHtml = p.children.map(function(c,i){
					var nm = c.name || ('Child ' + (i+1));
					var age = c.age ? (' (Age ' + c.age + ')') : '';
					return '<li>' + nm + age + '</li>';
				}).join('');
			} else if(Array.isArray(p)){
				listHtml = p.filter(function(c){ return c && (c.fn || c.ln || (c.dob && c.dob.trim() !== '')); })
							.map(function(c,idx){
								var full = ((c.fn||'').trim() + ' ' + (c.ln||'').trim()).trim();
								var namePart = full ? ('Name: ' + full) : ('Child ' + (idx+1));
								var dobPart = (c.dob && c.dob.trim() !== '') ? (' — DOB: ' + c.dob) : '';
								return '<li>' + namePart + dobPart + '</li>';
							}).join('');
			}
			if(listHtml){
				return '<div style="margin-top:6px;"><div style="font-weight:600;">Children on file</div><ul style="margin:6px 0 0 18px;">' + listHtml + '</ul></div>';
			}
		}catch(e){}
		return '';
	}

	var pnRaw = order && order.properties && order.properties.customerphone ? order.properties.customerphone : '';
	var digitsRaw = (pnRaw || '').replace(/\D/g,'');
	var digits = digitsRaw.length > 10 ? digitsRaw.slice(-10) : digitsRaw;
	if(!digits || digits.length < 10){
		// no valid phone to check
		$box.hide();
		return;
	}

	$box.show();
	if ($status.length) $status.text('Checking waiver status for ' + digits + ' ...').css('color','#777');
	$.post('/m/api/releaseWaiver_lookup', { hardcode: "iThinkThisIsSafeEnoughHaHaHaHa", restname: restShortName, pn: digits }, function (resp) {
		if(!resp || resp.error){
			if ($status.length) $status.text('Lookup failed. Please try again.').css('color','#b00020');
			return;
		}
		var html = '';
		var waiverBase = (location.hostname === '127.0.0.1' ? 'http://127.0.0.1:5001' : 'https://signup.restmesh.com') + '/releaseWaiver/';
		var restSeg = encodeURIComponent(restShortName || '');
		var extraParams = [];
		extraParams.push('cb=' + encodeURIComponent(window.location.href));
		extraParams.push('pn=' + digits);
		// Try to include email / first name / last name from order properties
		try {
			var props = (order && order.properties) || {};
			if (props.customeremail) extraParams.push('em=' + encodeURIComponent(props.customeremail));
			if (props.customerfirstname) extraParams.push('fn=' + encodeURIComponent(props.customerfirstname));
			if (props.customerlastname) extraParams.push('ln=' + encodeURIComponent(props.customerlastname));
		} catch(_e) {}
		var cbHref = waiverBase + restSeg + '?' + extraParams.join('&');
		var ctaBtn = '<div style="margin-top:10px;"><a href="' + cbHref + '" style="display:inline-block;background:#cf7e2e;color:#fff;border:none;border-radius:8px;padding:10px 14px;font-weight:600;cursor:pointer;text-decoration:none;">Sign waiver now</a></div>';
		if(resp.status === 'valid'){
			if ($status.length) $status.text('Waiver is valid within the last 12 months.').css('color','#0a6');
			if(resp.name) html += '<div><strong>Name:</strong> ' + resp.name + '</div>';
			if(resp.email) html += '<div><strong>Email:</strong> ' + resp.email + '</div>';
			if(resp.dob) html += '<div><strong>DOB:</strong> ' + resp.dob + '</div>';
			if(resp.lastTime){
				var dt = new Date(resp.lastTime);
				html += '<div><strong>Signed on:</strong> ' + (isNaN(dt)? resp.lastTime : dt.toLocaleDateString()) + '</div>';
			}
			html += renderKids(resp.prop);
			html += '<div style="margin-top:10px;"><a href="' + cbHref + '" style="display:inline-block;background:#cf7e2e;color:#fff;border:none;border-radius:8px;padding:10px 14px;font-weight:600;cursor:pointer;text-decoration:none;">Add/Edit waiver</a></div>';
		} else if(resp.status === 'expired'){
			if ($status.length) $status.text('Waiver on file has expired. Please re-sign.').css('color','#cc6e00');
			if(resp.lastTime){
				var dt2 = new Date(resp.lastTime);
				html += '<div><strong>Last signed on:</strong> ' + (isNaN(dt2)? resp.lastTime : dt2.toLocaleDateString()) + '</div>';
			}
			html += ctaBtn;
		} else {
			if ($status.length) $status.text('No waiver record found for this phone.').css('color','#444');
			html += ctaBtn;
		}
		// Party invite list from order properties
		try {
			var invites = (order && order.properties && Array.isArray(order.properties.partyInvites)) ? order.properties.partyInvites : [];
			if (invites.length) {
				var invHtml = '<div style="margin-top:12px;"><div style="font-weight:600;">Guest Waiver Status</div><div style="margin-top:6px;">';
				var tasks = [];
				invites.forEach(function(it, idx){
					var name = (it.name || '').trim();
					var email = (it.email || '').trim();
					var waiverId = it.waiverId || it.waiverID || null;
					var rowId = 'inv_' + idx + '_' + Math.random().toString(36).slice(2);
					var statusText = waiverId ? 'Checking...' : 'Not signed yet';
					var statusColor = waiverId ? '#777' : '#cc6e00';
					invHtml += '<div id="' + rowId + '" style="padding:6px 0; border-bottom:1px solid #eee;">' +
					           '<div><strong>' + (name || email || 'Guest') + '</strong> <span style="color:#666;">' + (email ? ('&lt;' + email + '&gt;') : '') + '</span></div>' +
					           '<div class="status" style="color:' + statusColor + '; font-size:12px;">' + statusText + '</div>' +
					           '</div>';
					if (waiverId) {
						tasks.push(function(done){
							$.post('/m/api/releaseWaiver_byId', { hardcode: 'iThinkThisIsSafeEnoughHaHaHaHa', restname: restShortName, waiverId: waiverId }, function(r){
								var $row = $('#' + rowId);
								if (r && r.status === 'ok') {
									var when = r.time ? new Date(r.time).toLocaleDateString() : '';
									$row.find('.status').css('color','#0a6').text('Signed' + (when ? (' on ' + when) : ''));
									// add details under row
									var details = [];
									if (r.email) details.push('Email: ' + r.email);
									if (r.dob) details.push('DOB: ' + r.dob);
									if (when) details.push('Signed on: ' + when);
									if (details.length) {
										var htmlDetails = '<div class="inv-details" style="color:#555;font-size:12px;margin-top:2px;">' + details.join(' · ') + '</div>';
										$row.append(htmlDetails);
									}
									var kidsHtml = renderKids(r.prop);
									if (kidsHtml) { $row.append('<div class="inv-children" style="font-size:12px;">' + kidsHtml + '</div>'); }
								} else {
									$row.find('.status').css('color','#cc6e00').text('WaiverId not found');
								}
								done();
							}).fail(function(){ $('#' + rowId).find('.status').css('color','#b00020').text('Lookup failed'); done(); });
						});
					} else {
						// no async task needed
					}
				});
				invHtml += '</div></div>';
				html += invHtml;
				if ($details.length) $details.html(html);
				// Run lookups sequentially to avoid burst, after DOM is in place
				(function run(i){ if (i>=tasks.length){ return; } tasks[i](function(){ run(i+1); }); })(0);
				return;
			}
		} catch(e) {}
		if ($details.length) $details.html(html);
	}).fail(function(){
		if ($status.length) $status.text('Lookup failed. Please check network.').css('color','#b00020');
	});
}


function promptAddToHomePage(){
	//add to home page
	$.templates.addToHome.link($("#fullpageWindow"), {});
	$("#fullpageWindow").css('display','block');

	$(".closePage").tap(function(){
		$("#fullpageWindow").css('display','none');
	});
  }

//check if there is cp value items in the sorteditems list, if so, move to last
function manuallyMoveCpRwdItems(obj){
	var afterMove = [];
	var indexList = [];
	for(var i = 0; i < obj.sortedItems.length; i++){
		if(obj.sortedItems[i].id == -3 || obj.sortedItems[i].id == -2){
			indexList.push(i);
		} else {
			afterMove.push(obj.sortedItems[i]);
		}
	} 
	for(var j = 0; j < indexList.length ;j++){
		var item = indexList[j];
		afterMove.push(obj.sortedItems[item]);  //push cp value to last 
	}
	obj.sortedItems = afterMove;
}

function renderTrackOrderPageNoOrderId() {
	var lastOrder = getLastOrder();
	if (typeof lastOrder == 'undefined' || lastOrder == null) {
		$("#container").html("You have not placed any togo order yet.");
	}
	else {
		loadRestaurantsData(function(data){
			currentRest = getRestaurantByInfo(lastOrder.restname, lastOrder.restid);
			var orderToken = lastOrder.orderid;
			$(window).scrollTop(0);
			History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));
		});
	}
}

function renderTrackOrderHeader(container) {
	currentRest.opened = inOpenHours(currentRest);

	if (!container) {
		container = $("#headerContainer");
	}

	container.html($.templates.trackorderheader.render({rest: currentRest}));
	container.css('margin-top', 0);

	$("#refresh").tap(function () {
		try {
			updateOrderStatus();
		} catch (e) {
			location.reload();
		}
	});
	$("#home").tap(function () {
		//History.pushState({render: "renderRestrauntsPage" }, "OrderToGo.com", currentRest.config.ordertogohomelink);
		window.location = currentRest.config.ordertogohomelink;
	});
}


function saveLastOrder(orderToken) {	
	var orderHistory = getOrderToGoHistory();
	if (orderHistory == null)
		orderHistory = [];
	orderHistory.unshift({
		order: orderToken.order,
		orderid: orderToken.orderid,
		orderToken: orderToken.order.orderToken,
	});
	if (orderHistory.length >= 11)
		orderHistory.pop();
	localStorage["orderToGoHistory"] = JSON.stringify(orderHistory);
}

function getOrderToGoHistory()
{
	var orderToGoHistory = localStorage["orderToGoHistory"];
	if (orderToGoHistory == null)
		return null
	return JSON.parse(orderToGoHistory);
}

function onOrderTrackerLoaded()
{
	$(document.body).css("max-width", "").css("margin", "");

	clearInterval(timer);
	//timer = setInterval(updateOrderStatus, 60 * 1000);
	updateOrderStatus();

	$("#backToHome").tap(function(){
		window.location="/";
	});
}

function getOrderToken(){
	var items = [];
		for (var i = context.cart.items.length - 1; i >= 0; i--) {
			items.push(context.cart.items[i].id);
		};
		var order = {
			restid: currentRest.id,
			items: items
		};

		$.ajax({
			url: "/m/api/orders",
			type: "POST",
			contentType: "application/json",
			data: JSON.stringify(order)
		})
		.done(function(data) {
				console.log(data);

				context.cart.orderToken = data.token;
				context.cart.tip = -1;
				saveCart(currentRest, context.cart);

				//gotoCheckoutPage();
			})
		.fail(function(reason) {
				console.log(reason);

				//clearCart();
				//alert("Oops! Something wrong happened, please retry.");
				if (reason.status == 498) // custom code 498, restaurant is close at the moment
				{
					//showRestaurantOpenSchedule();
					// showRestaurantClosed()
				}
				else
				if (reason.responseJSON)
					alert(reason.responseJSON.message);
				else
				if (reason.responseText)
					alert(reason.responseText);
		});
}

function getOrderDetails(){
	// instead of just item_id, include options 
	var orderdetails = {
		items: [],
		subtotal: 0.00
	}
	var subtotal = 0.00;
	for (var i = context.cart.items.length - 1; i >= 0; i--) {
		if(context.cart.items[i].optionsstr){
			var optionsstr = context.cart.items[i].optionsstr;
			if(context.cart.items[i].specialIns){
				optionsstr += "," + context.cart.items[i].specialIns
			} 
		} else {
			var optionsstr = null;
			if(context.cart.items[i].specialIns){
				optionsstr = context.cart.items[i].specialIns
			} 
		}	
 
		var optionItemIdsAndPrices = [];
		if(context.cart.items[i].optionitemids){
			var optionitemids = context.cart.items[i].optionitemids;
			for(var j = 0; j < context.cart.items[i].optionitemobjects.length; j++){
				var db_id = context.cart.items[i].optionitemobjects[j].db_id;
				var price = context.cart.items[i].optionitemobjects[j].price;
				var name = context.cart.items[i].optionitemobjects[j].name;
				optionItemIdsAndPrices.push({id: db_id, name: name ,p:price}); 
			} 
		} else {
			var optionitemids = null;
		}
		var item = {
			item_id: context.cart.items[i].id,
			optionsstr: optionsstr,
			optionitemids:  optionitemids,
			optionItemIdsAndPrices: optionItemIdsAndPrices,
			price: context.cart.items[i].price,  //price with Option
			taxrate: context.cart.items[i].taxrate,
		}  
		orderdetails.items.push(item);
		subtotal += item.price;
	};
	orderdetails.subtotal = subtotal;
	return orderdetails;
}

// only in togo
function onPlaceOrderLoaded() {
	$('#customerphone').mask('(000) 000-0000');
	$("#customername").attr('maxlength','30');

	$.get("/m/generateBrainTreeClientToken").done(function(clientToken) {
		braintree.setup(clientToken, "dropin", {
			container: "payment-form",
			onReady: function() {
				$('#buyButton').css("display", "");
				$('.showOnlyWhenBTLoaded').css("display", "");
			},
			onPaymentMethodReceived: function(payload) {
				// console.log("haha2");

				function actuallyCheckout()
				{

					var orderdetails = getOrderDetails();

					var postParams = {
						nonce: payload.nonce,
						tip: context.cart.tip,
						customerphone: context.customerphone,
						customername: context.customername,
						deliveryfee: context.deliveryfee,
						restname: currentRest.name,
						orderdetails: orderdetails,
						restid: currentRest.id,
						database : currentRest.database,
						// "waliUserId": window.waliUserId
					}; 
					if (context.deliverymethod == 1 && context.deliveryaddress != null)
					{
						postParams.deliveryaddress = context.deliveryaddress;	// this should be the address that is already formatted by our geo api
					}

					//get orderToken here instead of creatingPendingOrder
					//getOrderToken();

					var url = "/m/api/orders/braintreeCheckout";
					var data = JSON.stringify(postParams);
					$.ajax({
						'type': 'POST',
						'url': url,
						'contentType': 'application/json',
						'async': true,
						'data': data,
						'headers': { '__requestid': "" + (new Date()).getTime() + "_" + Math.floor((Math.random() * 10000)) },
						//'headers': { 'Authorization': base.jwtToken, '__requestid': "" + (new Date()).getTime() + "_" + Math.floor((Math.random() * 10)) },
						'success': function (data) {
							
								var orderToken = data.orderToken; 
								clearCart();
								saveLastOrder({orderid: data.orderid, order: data});
								var redirect = getURLParameterByName("r");
								if (redirect)
								{
									var redirectURL = redirect + "?l=" + orderToken;
									window.location.href = redirectURL;
									
								} else
								{
									$(window).scrollTop(0);
									History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));				
								}
							
						}
					}).fail(function (jqXHR, textStatus, errorThrown) {
						// function(data){
						// 	// TODO: figure out a best way to mark failed order (if necessary).
						// 	//alert("Failed to complete your payment, please retry.");
							
						// 	if (data.status == 498) // custom code 498, restaurant is close at the moment
						// 	 {
						// 		 showRestaurantOpenSchedule();
						// 		 //showRestaurantClosed()
						// 	 }
						// 	 else
						// 	if (data.responseJSON)
						// 		 alert(data.responseJSON.message);
						// 	 else
						// 	 if (data.responseText)
						// 		 alert(data.responseText);
						// }
					});
					// .always(function (jqXHR, textStatus, errorThrown) {
					// 	if (always) {
					// 		always(jqXHR, textStatus, errorThrown);
					// 	}
					// });

					// 5/4/2018: we now use the above $.ajax instead as we found out that $.post will converts all floats into strings when the server recives the posted object
					// $.post("/m/api/orders/braintreeCheckout", postParams
					// ).success(function(data){
					// 	var orderToken = context.cart.orderToken;
					// 	clearCart();
					// 	saveLastOrder({orderid: orderToken, order: data});
					// 	var redirect = getURLParameterByName("r");
					// 	if (redirect)
					// 	{
					// 		var redirectURL = redirect + "?l=" + orderToken;
					// 		window.location.href = redirectURL;
							
					// 	} else
					// 	{
					// 		$(window).scrollTop(0);
					// 		History.pushState({render: "renderOrderTracker", arguments: [orderToken] }, "Order placed", "/trackorder/" + encodeURIComponent(orderToken));				
					// 	}
					// }).error(function(data){
					// 	// TODO: figure out a best way to mark failed order (if necessary).
					// 	//alert("Failed to complete your payment, please retry.");
						
					// 	if (data.status == 498) // custom code 498, restaurant is close at the moment
					//  	{
					// 		 showRestaurantOpenSchedule();
					// 		 //showRestaurantClosed()
					//  	}
					//  	else
					// 	if (data.responseJSON)
					//  		alert(data.responseJSON.message);
					//  	else
					//  	if (data.responseText)
					//  		alert(data.responseText);
					// })
				}				

				// Need to verify the address
				// if (context.deliverymethod == 1)
				// {
				// 	// delivery, so pop up the address confirmation dlg

				// 	if (currentRest.config.centercoord == null || currentRest.config.centercoord.length == 0 || JSON.parse(currentRest.config.centercoord) == null)
				// 	{
				// 		alert("System admin: restaurant centercoord is missing.");
				// 		return;
				// 	}

				// 	dlgWithDeliveryAddress(context.deliveryaddress, function(){
				// 		context.deliveryaddress = data.results[0].formatted_address;
				// 		$('#modalConfirm').modal("hide");
				// 		actuallyCheckout();
				// 	});			
				// } else
				{
					// pickup only, so checkout directly
					actuallyCheckout();
				}
			}
		});
	});

	$("#showPolicy").tap(function(){
		$("#policyText").toggleClass("hidetext");
	});

	$(".btn-tip-percent").tap(function(e){

		$(".btn-tip-percent").removeClass("tip-active");
		$(this).toggleClass('tip-active');

		var tipPercent = $(this).attr("value");
		$.observable(context.cart).setProperty("tip", precise_round(context.cart.subtotal * (+tipPercent), 2));
		e.preventDefault();

		var dropdown = $(this).parents(".btn-group");
		dropdown.removeClass("open");
	});

	if (context.rest.config.enabledelivery)
	{
		$(".btn-pickup-deliver").tap(function(e){
			var deliverymethod = parseInt($(this).attr("value"));
			$.observable(context).setProperty("deliverymethod", deliverymethod);		
			e.preventDefault();

			var dropdown = $(this).parents(".dropdown");
			dropdown.removeClass("open");

			if (context.deliverymethod == 1)
			{
				// deliver
				$("#deliveryaddressform").css("display", "");
			} else
			{
				$("#deliveryaddressform").css("display", "none");
			}
		});

		// $("#deliveryaddress").tap(function(){
		// 	var str = $("#deliveryaddress").html();
		// 	BootstrapDialog.show({
	    //         title: 'Input Delivery Address',
	    //         message: $('<textarea id="addressInput" class="form-control" placeholder="Delivery Address">' + (str == "Click to input delivery address" ? "" : str) + '</textarea>'),
	    //         buttons: [{
	    //             label: 'Confirm',
	    //             cssClass: 'btn-primary',
	    //             hotkey: 13, // Enter.
	    //             action: function(dialogItself) {	                    
	    //                 dialogItself.close();

	    //                 // delivery, so pop up the address confirmation dlg
		// 				if (currentRest.config.centercoord == null || currentRest.config.centercoord.length == 0 || JSON.parse(currentRest.config.centercoord) == null)
		// 				{
		// 					alert("System admin: restaurant centercoord is missing.");
		// 					return;
		// 				}

        //             	var address = $("#addressInput").val();
        //             	dlgWithDeliveryAddress(address, function(deliveryfee, formatted_address){
		// 			    	$.observable(context).setProperty("isDeliveryFeeReady", true);
		// 			    	$.observable(context).setProperty("deliveryfee", deliveryfee);
		// 			    	$.observable(context).setProperty("deliveryaddress", formatted_address);
		// 			    	$("#deliveryaddress").html(formatted_address);
		// 			    });
	    //             }
	    //         }]
	    //     });
		// });

		// $("#deliveryaddress").editable({
		// 	showbuttons: "bottom",
		// 	//mode: "inline",
		// 	//onblur: "ignore",
		// 	success: function(response, newValue) {			    
		// 	    //context.deliveryaddress = newValue;
		// 	    var str = (' ' + newValue).slice(1); // string deep copy
		// 	    dlgWithDeliveryAddress(str, function(deliveryfee, formatted_address){
		// 	    	$.observable(context).setProperty("isDeliveryFeeReady", true);
		// 	    	$.observable(context).setProperty("deliveryfee", deliveryfee);
		// 	    	$.observable(context).setProperty("deliveryaddress", formatted_address);
		// 	    	//context.deliveryfee = deliveryfee;
		// 	    	//context.deliveryaddress = formatted_address;			    	
		// 	    	$("#deliveryaddress").html(formatted_address);
		// 	    });
		// 	},
		// 	// display: function(value, sourceData) {
		// 	// 	$("#deliveryaddress").removeClass("editable-empty");
		// 	// }
		// });
		//$("#deliveryaddress").html("Click to input delivery address");
	} else
	{
		$.observable(context).setProperty("deliverymethod", 0 /*0 means pickup, 1 means delivery*/);
	}	
}

//only for togo
function checkBeforePlaceToGoOrder(){
	// check if the user selected a tip
	if (context.cart.tip < 0)
	{
		shakeDomObject($('#tipGroup'));
		return false;
	}

	var btnSelectDeliveryMethod = $('#btn-selectdeliverymethod');
	if (context.deliverymethod == null && btnSelectDeliveryMethod.length > 0)
	{
		shakeDomObject(btnSelectDeliveryMethod);
		return false;
	}

	if (context.deliverymethod == 1)
	{
		var deliveryaddress = $('#deliveryaddress');
		//context.deliveryaddress = deliveryaddress.val();
		if (context.deliveryaddress == null || context.deliveryaddress.length == 0 || !context.isDeliveryFeeReady)
		{
			shakeDomObject(deliveryaddress);
			return false;
		}		
	}

	var phone = $('#customerphone').val().replace(/\D/g,'');
	if (phone.length == 10)	// TODO: we only accept 10 digit phone number at this point
	{		
		context.customerphone = $('#customerphone').val();
		context.customername = $('#customername').val();				
	} else
	{
		//alert("Please input a valid phone number");
		shakeDomObject($('#customerphone')); // mandatory field is missing, scroll to it, shake it and focus on it 
		return false;
	}	

	return true;
}

//only for togo
function getURLParameterByName(name, url) {
    if (!url) url = window.location.href;
    //url = url.toLowerCase(); // This is just to avoid case sensitiveness  
    name = name.replace(/[\[\]]/g, "\\$&");//.toLowerCase();// This is just to avoid case sensitiveness for query parameter name
    var regex = new RegExp("[?&]" + name + "(=([^&#]*)|&|#|$)"),
        results = regex.exec(url);
    if (!results) return null;
    if (!results[2]) return '';
    return decodeURIComponent(results[2].replace(/\+/g, " "));
}

//only for togo, not being used at this point
function dlgWithDeliveryAddress(deliveryaddress, confirm, cancel)
{
	// the Haversine formula
	function getDistance(p1, p2) {
	  var rad = function(x) {
		  return x * Math.PI / 180;
		};
	  var R = 6378137; // Earth’s mean radius in meter
	  var dLat = rad(p2.lat - p1.lat);
	  var dLong = rad(p2.lng - p1.lng);
	  var a = Math.sin(dLat / 2) * Math.sin(dLat / 2) +
	    Math.cos(rad(p1.lat)) * Math.cos(rad(p2.lat)) *
	    Math.sin(dLong / 2) * Math.sin(dLong / 2);
	  var c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
	  var d = R * c;
	  return d; // returns the distance in meter
	};

	function onBoxShown()
	{
		$("#explainInfo").tap(function(){
			BootstrapDialog.show({
				message: 'We use Google Maps to look up your delivery address and then calculate the delivery fee. The delivery address you see here is the sanitized version based on your input. The delivery driver will also use this sanitized address because it is more GPS friendly and thus can increase delivery accuracy.'
			});
		});
	}

	function onBoxShownFail()
	{
		$("#explainInfo").tap(function(){
			BootstrapDialog.show({
				message: 'We use Google Maps to look up your delivery address and then calculate the delivery fee. If we cannot find your delivery address, most likely the address is not GPS friendly and we cannot garantee the delivery driver could find you. Please use a different address that is GPS friendly.'
			});
		});
	}

	var container = $('#modalConfirm');
	var ctx = {
		title: "Looking up your delivery address...",
		addressValid: false
	};
	$.templates.confirmDeliveryAddress.link(container, ctx);
	container.modal('show');
						
	var maxDeliveryDistance = 4580;
	if (currentRest.config.deliverymaxdist)
		maxDeliveryDistance = parseFloat(currentRest.config.deliverymaxdist) * 1609.34; // convert from miles to meters
	$.post("/m/api/mapping/geocode", {
		address: deliveryaddress
	}).success(function(data){
		//console.log(data);

		var modalInfo = $("#modalInfo");
		var center = JSON.parse(currentRest.config.centercoord);//{lat: 47.6189955, lng: -122.1934885};	// this is shsh
		if (data.status == "OK")
		{
			var deliverTo = data.results[0].geometry.location;
			var distance = getDistance(center, deliverTo);
			if (distance <= maxDeliveryDistance)
			{
				var miles = precise_round(distance * 0.000621371, 2);  // distance is in meters, so we convert it into miles
				var dist = miles; // dist will be referenced in currentRest.config.deliveryfee below
				var fee = precise_round(eval(currentRest.config.deliveryfee), 2);
				$.observable(ctx).setProperty("title", "<div class='text-success'>Great, you are within our delivery radius!</div><mark>Delivery fee: $ " + precise_round_str(fee, 2) + "</mark>        <small>Distance: " + precise_round_str(miles, 2) + " miles.</small>");
				$.observable(ctx).setProperty("addressValid", true);								
				modalInfo.html("Restaurant address Point A. Please confirm<br>" + 
							"Delivery address Point B: <mark>" + data.results[0].formatted_address + "</mark>&nbsp&nbsp<small style='text-decoration: underline;color: royalblue;' id='explainInfo'>Why this might look different from what I just typed?</small>");
				$(".modal-footer .btn-success").tap(function(){
					if (confirm)
					{
						confirm(fee, data.results[0].formatted_address);
						container.modal('hide');
					}
				});
			} else
			{
				$.observable(ctx).setProperty("title", "<div class='text-danger'>Sorry, you are outside our delivery radius</div><small>Please select pickup or use a different delivery address</small>");								
				modalInfo.html("Point A is the restaurant. Your delivery address is at<br>" + 
							"Point B: <mark>" + data.results[0].formatted_address) + "<mark>"
			}							
			
			$('#map').css('height', Math.min(window.innerHeight - 255, 400));
			var map = new google.maps.Map(document.getElementById('map'), {
			    zoom: 11,
			    center: center,
			    mapTypeId: google.maps.MapTypeId.ROADMAP
			});
			var marker = new google.maps.Marker({
	          position: center,
	          label: "A",
	          map: map
	        });
	        var cityCircle = new google.maps.Circle({
		      strokeColor: '#5cb85c',
		      strokeOpacity: 0.5,
		      strokeWeight: 2,
		      fillColor: '#5cb85c',
		      fillOpacity: 0.2,
		      map: map,
		      center: center,
		      radius: maxDeliveryDistance
		    });
			new google.maps.Marker({
	          position: data.results[0].geometry.location,
	          label: "B",
	          map: map
	        });

	        onBoxShown();
		} else
		{
			// geocode failed
			$.observable(ctx).setProperty("title", "Cannot look up your address");
			if (data.error_message != null)
				modalInfo.html(data.error_message);
			else
				modalInfo.html("");

			modalInfo.html( modalInfo.html() + "&nbsp&nbsp<small style='text-decoration: underline;color: royalblue;' id='explainInfo'>More info</small>" );

			onBoxShownFail();
		}
	}).error(function(data){
		//console.log("error");
		var modalInfo = $("#modalInfo");
		$.observable(ctx).setProperty("title", "Cannot look up your address");
		modalInfo.html("Failed to find your delivery address.");
		modalInfo.html( modalInfo.html() + "&nbsp&nbsp<small style='text-decoration: underline;color: royalblue;' id='explainInfo'>More info</small>" );

		onBoxShownFail();
	});		
}
