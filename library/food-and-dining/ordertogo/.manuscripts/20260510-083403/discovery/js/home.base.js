//for home 
function onRestaurantsLoaded() {
	// $(document.body).css("max-width", "768px").css("margin", "auto");
	//lateInitFacebookShare();
	var visited = localStorage['visited'];
	if (visited == 'yes') {
			// second page load, cookie active
	} else {
		openPopup(); // first page load, launch fancybox
	}
	localStorage['visited'] = 'yes';
		
	$(".rest").tap(function(event) {
		var index = $(this).attr("index");
		showChooseDineinOrTogo(index);
	});

	var q = getQueryParams(document.location.search);
	if (q.cover)
	{
		for (var i = 0; i < restaurants.length; ++i)
		{
			if (restaurants[i].name == q.cover)
			{
				if (q.tid || q.secm == "1")
				{
					var url = '/m/api/restaurants/' + restaurants[i].name + '/tables'
					$.get(url)
					.done(function(data) {
						$.post("m/api/dinningtablesSto",
						{
							hardcode: "iThinkThisIsSafeEnoughHaHaHaHa",
							restid: restaurants[i].id,
							restname:restaurants[i].name,
							encryptOrderId: true
						}).done(function(tablesData){
							if (q.tid)
							{
								var realTid = q.tid.split('_')[0];
								var tableName = data[realTid];
								if (typeof tableName != 'undefined')
								{
									showChooseDineinOrTogo(i, true, tableName, q.tid, tablesData[realTid]);
								}
							} else
							if (q.secm == "1")
							{
								var temp = [{id: 0, name: "Select a table..."}];
								for (var key in data)
								{
									var name = data[key];
									if (name.match(/order to go/i) == null && name != "")
										temp.push({id: key, name: name});
								}
								showChooseDineinOrTogo(i, true, temp);
							}
						});
					});
					break;
				} else
				{
					showChooseDineinOrTogo(i, true);			
					break;
				}
			}
		}
	}
}



function showChooseDineinOrTogo(index, noAnim, tableName, tidIn, tableData)
{
	var target = restaurants[index];

	function gotoMenuPage(target)
	{		
		if (target.config.ordertogohomelink && target.config.ordertogohomelink.length > 0)
			window.location = target.config.ordertogohomelink;
		else
			History.pushState({ render: "renderMenuPage", arguments: [target] }, target.fullname + " - OrderToGo.com", "/restaurants/" + encodeURIComponent(target.domainname));
	}		
	
	//if (window._isMobile)
	delete localStorage._tableSelected;
	if (true)
	{
		//scantoorder
		var container = $('#modalConfirm');
		
		if (typeof tableName == "object")
		{
			// tableName is an object
			var ctx = {
				rest: target,
				tableNames: tableName,
				enableScanToOrder: target.enableScanToOrder == null ? false : target.enableScanToOrder				
			};
			$.templates.chooseDineinOrTogoTableNames.link(container, ctx);

			$("#tableDropDown").change(function(){
				// get tid
				delete localStorage._tableSelected;
				var val = parseInt($(this).val());
				if (val != 0)
				{
					$.post("/m/api/gettabletid", {tid: val, restname: target.name, /*code: */})
					.done(function(data){
						localStorage._tableSelected = JSON.stringify({
							tid: val + "_" + data.tid,
							name: $("#tableDropDown option:selected").text(),
							restname: target.name
						});
					});
				}
			});

			container.modal({
				show: true,
				backdrop: 'static'
			});
		} else
		{
			// tableName is null or string
			var ctx = {
				rest: target,
				tableName: tableName,
				hasTable: tableName == null ? false : true,
				enableScanToOrder: target.enableScanToOrder == null ? false : target.enableScanToOrder,
				enableToGo: target.enableToGo == null ? false : target.enableToGo
			};
			$.templates.chooseDineinOrTogo.link(container, ctx);

			// 4/10/2019: this bypasses dine in confirm code checking
			if (tableName && tidIn)
			{
				localStorage._tableSelected = JSON.stringify({
					tid: tidIn,
					name: tableName,
					restname: target.name
				});
			}
			
			container.modal({
				show: true				
			});
		}

		if (window._isMobile && !noAnim)
			$('#scan2order-startpage').addClass('show');
		else
			$('#scan2order-startpage').addClass('showNoAnim');

		$("#openHoursToday-chooseBoxpage").text(getHoursToday(target));
		var isUsedOrder = false;
		if (tableData && tableData.order_id != -1) {
			isUsedOrder = true;
			$('#orderDinein-btn').text("Order More");			
			$("#tablePay-btn").tap(function () {
				if (/ordertogo/.test(location.host)) {
					location.href = "https://www.ordertogo.com/restaurants/" + target.name + "/mesh?pid=" + tableData.order_id;
				} else {
					location.href = location.origin + "/restaurants/" + target.name + "/mesh?pid=" + tableData.order_id;
				}
			});
		}
		
		$('#orderDinein-btn').tap(function(){
			var tableDropDown = $("#tableDropDown");
			if (tableDropDown.length > 0)
			{
				if (tableDropDown.val() == "0" || (typeof localStorage._tableSelected == "undefined"))
				{
					var ctx = {
						//alerttype: data.alerttype,
						title : "Info",
						msg1 : "Please select a table first.",
						//msg2 : data.msg2
					}
					var container2 = $('#modalConfirm-2');
					$.templates.customAlert.link(container2, ctx);
					container2.modal('show');
					return;
				}
			}

			var query = window.location.search;
			var queryStr = '';
			if (query != null && query.length > 0)
			{
				var queryCxt = query.substr(1);
				queryArr = queryCxt.split('&');
				for (var i = 0; i < queryArr.length; i++)
				{
					if (queryArr[i].indexOf('cover') != -1)
					{
						queryArr.splice(i, 1);
						break;
					}
				}
				queryStr = queryArr.join('&');
				if (queryStr.length > 0)
				{
					queryStr = '?' + queryStr;
				}
			}
			
			if (queryStr.indexOf("secm") == -1) {
				if (isUsedOrder) {
					window.location = "/restaurants/" + encodeURIComponent(target.domainname) + "/dinein" + queryStr + '&isUsedOrder=true&order_id=' + tableData.order_id;
				}
				else {
					window.location = "/restaurants/" + encodeURIComponent(target.domainname) + "/dinein" + queryStr;
				}
			}
			else
			{				
				window.location = "/restaurants/" + encodeURIComponent(target.domainname) + "/dinein";
			}
		});
		
		$('#orderViewOrder-btn').tap(function(){
			var _url = new URL(window.location.href);
			_url.searchParams.set("showHistory", "true");
			window.history.pushState({}, '', _url.toString());
			$('#orderDinein-btn').click();
		});

		$('#orderTogo-btn').tap(function(){
			container.empty();
			container.modal("hide");
			gotoMenuPage(target);
		});	

		$('#closeChooseBox').tap(function(){
			// container.modal('hide');
			closeDineinOrTogo()
		});
	} else
	{			
		gotoMenuPage(target);
	}

	if (!tableData || tableData.order_id == -1) {
		$("#tablePay-btn-row .scan2order-text").append("<span> (No Order Yet)</span>");
		$(".scan2order-btnContainer .btn-warning").css("background-color","#bbbab8").css("border-color","#bbbab8");
	}

	if (window.AfterShowChooseDineinOrTogo)
		window.AfterShowChooseDineinOrTogo(tableData);
}

//for home
function closeDineinOrTogo(){
	//remove tabel id
	delete localStorage._tableSelected;
	window.location = "/";

    var container = $('#modalConfirm');
    $('#scan2order-startpage').removeClass('show');
    setTimeout(function() { container.modal('hide'); }, 240);
}

//**********  used in admin.togo.order.list.js  
function loadTemplate(name, path) {
    var deferred = $.Deferred();
    if ($.templates[name]) {
      deferred.resolve();
    } else {
        if (!path) {
            path = "/templates/" + name + ".tmpl"
        }
  
      $.get(path)
       .done(function(data) {
           $.templates(name, data);
          deferred.resolve();
        });
    }
    return deferred.promise();
}



function loadScript(file, callback1, callback2) {
	var head = document.getElementsByTagName('head')[0];
	var script = document.createElement('script');
	script.type = 'text/javascript';
	script.src = file;
	head.appendChild(script);
	script.onload = script.onreadystatechange = function () {
		if (!this.readyState || this.readyState === "loaded" || this.readyState === "complete") {
			script.onload = script.onreadystatechange = null;
			console.log(file.substring(file.lastIndexOf('/') + 1) + ' loaded');
			if (callback1)
				callback1();
			if(callback2)
				callback2();
		}
	}
}

function showKeyboardCB(inputArea, callback) {
	if(location.href.indexOf('selforderkiosk') < 0) return;
	if($('#keyboardContainer').length && $('#keyboardContainer').css('display') != 'none') return;
	// load tmpl & js files
    if($.templates.keyboard == null) {
        function loadKeyboardTmpl() {
            return loadTemplate("keyboard", "/templates/keyboard.tmpl");
        }
        $.when(loadKeyboardTmpl()).done(function() {
            if(document.querySelector('script[src="/javascripts/keyboard.js"]') == null) {
                loadScript("/javascripts/keyboard.js", function() {
					showKeyboard(inputArea);
                    if(callback) callback();
                });
            }
        })
    }
    // files exist
    else {
		showKeyboard(inputArea);
        if(callback) callback();
    }
}

function getQueryParams(qs) {
    qs = qs.split('+').join(' ');

    var params = {},
        tokens,
        re = /[?&]?([^=]+)=([^&]*)/g;

    while (tokens = re.exec(qs)) {
        params[decodeURIComponent(tokens[1])] = decodeURIComponent(tokens[2]);
    }

    return params;
}

function sortItems(data){
	function compare(a, b) {
		if (a.item_id < b.item_id)
			return -1;
		if (a.item_id > b.item_id)
			return 1;
		return 0;
	}

	var cate_obj = {};
	var orderMap = [];
	for(var a_item of data)
	{
		var t = a_item.item_id.length - 1;
		for (; t >= 0; t--)
		{
			if (!(a_item.item_id[t] >= '0' && a_item.item_id[t] <= '9'))
				break;
		}		
		var cate_str = a_item.item_id.substring(0, t + 1);
		if(cate_obj[cate_str] == null)
		{
			orderMap.push(cate_str);
			cate_obj[cate_str] = [];
		}
		cate_obj[cate_str].push(a_item);
	}

	for(var arr in cate_obj)
	{
		cate_obj[arr].sort(compare);
	}

	// sort all by dishcategorymapping
	var dishcategorymapping = {}
	if (currentRest.config.dishcategorymapping == '' || currentRest.config.dishcategorymapping == null)
		dishcategorymapping = {}
	else if (typeof currentRest.config.dishcategorymapping == "string")
		dishcategorymapping = JSON.parse(currentRest.config.dishcategorymapping);
	else if (typeof currentRest.config.dishcategorymapping == "object")
		dishcategorymapping = currentRest.config.dishcategorymapping;
	var temp_orderMap = [];
	for (var key in dishcategorymapping) {
		var value = dishcategorymapping[key];
		if (!Array.isArray(value)) continue;
		var index = value[1];
		temp_orderMap.push([key, index]);
	}
	// sort dishcategorymapping
	temp_orderMap.sort(function(a, b) {
		return a[1] - b[1];
	});
	// get sorted orderMap
	var new_orderMap = [];
	for (var _ of temp_orderMap) {
		if (Array.isArray(_) && orderMap.includes(_[0]))
			new_orderMap.push(_[0]);
	}
	//fill missing value
	for (var _ of orderMap) {
		if (!new_orderMap.includes(_))
			new_orderMap.push(_);
	}
	if (new_orderMap.length == orderMap.length)
		orderMap = new_orderMap;
	// end sort all by dishcategorymapping

	var tmp_data = [];
	for(var arr of orderMap)
	{
		for(var a_item of cate_obj[arr])
		{
			tmp_data.push(a_item);
		}
	}
	return tmp_data;
}

function getOrderWithDetailsByIdResultToClientOrderFormat(full_menus, order){
	order.items = [];
	order.isPartyItem = false;
	var jsonMenu = {};
	for(var menu of full_menus)
	{
		/* menu =
		"id": 3296,
		"item_id": "HS02",
		"name": "full 葱油饼Green Onion Pancake",
		"price": 2.95,
		"options": "{\"opts\":[{\"name\":\"要啥\",\"values\":[{\"db_id\":\"3371\",\"isDefault\":false,\"name\":\"full多油\",\"price\":\"\",\"printer\":\"3\",\"printname\":\"print多油\"},{\"db_id\":\"3374\",\"isDefault\":false,\"name\":\"多葱\",\"price\":\"\",\"printer\":\"1\",\"printname\":\"\"},{\"db_id\":\"3375\",\"isDefault\":false,\"name\":\"少葱\",\"price\":\"\",\"printer\":\"1\",\"printname\":\"\"}],\"stepNum\":null,\"gotoN\":null,\"required\":true,\"uv\":true},{\"name\":\"option\",\"values\":[{\"db_id\":9169,\"isDefault\":false,\"name\":\"fullnewOption\",\"price\":\"2\",\"printer\":\"3\",\"printname\":\"printnewOption\"},{\"db_id\":\"\",\"isDefault\":false,\"name\":\"1\",\"price\":\"\",\"printer\":\"-1\",\"printname\":\"\"}],\"stepNum\":null,\"gotoN\":null,\"required\":false,\"uv\":true},{\"name\":\"option\",\"values\":[{\"db_id\":\"\",\"isDefault\":false,\"name\":\"newOption\",\"price\":\"\",\"printer\":\"-1\",\"printname\":\"\"},{\"db_id\":\"3377\",\"isDefault\":false,\"name\":\"2\",\"price\":\"1\",\"printer\":\"-1\",\"printname\":\"2\"}],\"stepNum\":null,\"gotoN\":null,\"required\":false,\"uv\":true},{\"name\":\"加料\",\"values\":[{\"db_id\":\"\",\"isDefault\":false,\"name\":\"葡萄\",\"price\":\"\",\"printer\":\"\",\"printname\":\"\"},{\"db_id\":3381,\"isDefault\":false,\"name\":\"苹果\",\"price\":\"2\",\"printer\":\"\",\"printname\":\"\"}],\"stepNum\":null,\"gotoN\":null,\"required\":false,\"uv\":true}],\"optCatNum\":{},\"optionItemIds\":[\"\"],\"groupItems\":[],\"allowInOrderToGo\":true,\"allowScanToOrder\":true,\"autoOptionPopup\":\"1\",\"itemColor\":\"\",\"image\":\"https://s3-us-west-2.amazonaws.com/dishimg.ordertogo.com/dz5/1519867660329_square.jpg\",\"descriptions\":\"Pan-fried wheat pancake with green onion\",\"itemsubtitle\":\"\",\"dishHour\":\"\",\"dishDate\":\"\",\"printersToSendTo\":\"1,2\",\"printExtra\":false,\"dontPrint\":false,\"itemMinNum\":\"\",\"barcodeType\":\"\",\"barcodeData\":\"\",\"costPrice\":\"\",\"dishPricesByTime\":\"[1,10]\",\"maxQuantRestriction\":0}",
		"taxrate": null,
		"itemsubtitle": "",
		"category": "点心"
		*/
		jsonMenu[menu.id] = menu;
	}
	context.menus = jsonMenu;
	for (var detail of order.details) {
		/* detail =
		item_id: 3296
		item_price: 16.95
		options: "少葱"
		time: 1584666750886
		item_type: 0
		properties: "{"modifiers":[{"id":"9169","name":"fullnewOpti
		*/
		var optionsstr;
		if (detail.options && detail.options.includes("✔︎")) {
			optionsstr = detail.options.split("✔︎")[0];
		} else {
			optionsstr = detail.options;
		}
		// need
		var lMenu = jsonMenu[detail.item_id] || {};
		try {
			var _options = JSON.parse(lMenu.options);
			if (_options.isPartyItem) {

				order.isPartyItem = _options.isPartyItem;
			}
		} catch (err) { };
		if (detail.item_id == -2 || detail.item_id == -3) {
			order.disableClientRewardAndCouponOption = true;
		}
		var lItem = {
			id: detail.item_id,
			item_id: lMenu.item_id,
			name: lMenu.name,
			price: detail.item_price,
			optionsstr: optionsstr,
			taxrate: lMenu.taxrate,
		}
		var lArrModifiers = JSON.parse(detail.properties || '{}').modifiers || [];
		var optionitemobjects = [];
		for (var modifier of lArrModifiers) {
			/*
			id: "9169",
			name: "fullnewOption",
			p: "2";
			*/
			optionitemobjects.push({
				name: modifier.name,
				price: parseFloat(modifier.p),
				db_id: modifier.id,
				printer: null,
				count: null
			});
		}
		var lArrRewardsInfo = JSON.parse(detail.properties || '{}').rewardsInfo || null;
		lItem.rewardsInfo = lArrRewardsInfo;

		lItem.optionitemobjects = optionitemobjects;
		order.items.push(lItem);
	}
	order.adjustment = order.invoiceadj;
}

function showAds(otgAdsImgList) {
	// random pick 1 from img list
	var _imgs = otgAdsImgList.split(';;');
	if (_imgs.length) {
		disableBodyScroll();
		var _img = _imgs[Math.floor(Math.random() * _imgs.length)];
		$('#adsDivContentImg').attr('src', _img);
		$('#adsDiv').show();
		$('#adsDivCloseImg').tap(function() {
			$('#adsDiv').hide();
			enableBodyScroll();
		});
	}
}