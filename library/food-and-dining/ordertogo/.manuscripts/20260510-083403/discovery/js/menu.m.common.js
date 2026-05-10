// 9/6/2019 miyu: Clearn up for 3rd Gen. This file contains all the common code used in any page with menu
// include selforder / dinein / mesh / table / order 

if (!String.prototype.includes) {
	String.prototype.includes = function(search, start) {
	  'use strict';
	  if (typeof start !== 'number') {
		start = 0;
	  }
	  
	  if (start + search.length > this.length) {
		return false;
	  } else {
		return this.indexOf(search, start) !== -1;
	  }
	};
  }

if (!String.prototype.replaceAll) {
	String.prototype.replaceAll = function(str, newStr){

		// If a regex pattern
		if (Object.prototype.toString.call(str).toLowerCase() === '[object regexp]') {
			return this.replace(str, newStr);
		}

		// If a string
		return this.replace(new RegExp(str, 'g'), newStr);

	};
}

if (!String.prototype.repeat) {
	String.prototype.repeat = function(count) {
	  'use strict';
	  if (this == null) {
		throw new TypeError('can\'t convert ' + this + ' to object');
	  }
	  var str = '' + this;
	  count = +count;
	  if (count != count) {
		count = 0;
	  }
	  if (count < 0) {
		throw new RangeError('repeat count must be non-negative');
	  }
	  if (count == Infinity) {
		throw new RangeError('repeat count must be less than infinity');
	  }
	  count = Math.floor(count);
	  if (str.length == 0 || count == 0) {
		return '';
	  }
	  // Ensuring count is a 31-bit integer allows us to heavily optimize the
	  // main part. But anyway, most current (August 2014) browsers can't handle
	  // strings 1 << 28 chars or longer, so:
	  if (str.length * count >= 1 << 28) {
		throw new RangeError('repeat count must not overflow maximum string size');
	  }
	  var maxCount = str.length * count;
	  count = Math.floor(Math.log(count) / Math.log(2));
	  while (count) {
		 str += str;
		 count--;
	  }
	  str += str.substring(0, maxCount - str.length);
	  return str;
	}
  }

if (!String.prototype.padStart) {
    String.prototype.padStart = function padStart(targetLength,padString) {
        targetLength = targetLength>>0; //truncate if number or convert non-number to 0;
        padString = String((typeof padString !== 'undefined' ? padString : ' '));
        if (this.length > targetLength) {
            return String(this);
        }
        else {
            targetLength = targetLength-this.length;
            if (targetLength > padString.length) {
                padString += padString.repeat(targetLength/padString.length); //append to original to ensure we are longer than needed
            }
            return padString.slice(0,targetLength) + String(this);
        }
    };
}

var number2Day = {
	1: "Monday",
	2: "Tuesday",
	3: "Wednesday",
	4: "Thursday",
	5: "Friday",
	6: "Saturday",
	0: "Sunday"
};

//common use for all hjs. (mesh / dinein / mobile / selforder / self checkout / tablet order )
function registerConverters() {
	$.views.converters({
		showlastfour: function(cardnumber)
		{
            return "••••" + cardnumber.replace(/\d(?=\d{4})/g, "");
        },
		disableWhenTrue: function(x)
		{
			if (x)
				this.linkCtx.elem.setAttribute("disabled", "1");
			else
				this.linkCtx.elem.removeAttribute("disabled");
		},
		imgURLFromMenuOptionsStr: function(options)
		{
			var opt = null;
			if (options != null)
				opt = JSON.parse(options);
			
			if (opt == null)
				return "https://s3.us-west-2.amazonaws.com/dishimg.ordertogo.com/demo2019/1573170514093_square.jpg";
			
			if (!opt.image)
				return "https://s3.us-west-2.amazonaws.com/dishimg.ordertogo.com/demo2019/1573170514093_square.jpg";	

			return opt.image;
		},
		highLightOrderToGoHistoryRow: function(orderToken)
		{
			return (context.orderToken == orderToken) ? "dzYellowBg" : "";
		},
		toDateTime: function(time)
		{
			if (time == null) return "";
			var date = new Date(time);				
			return timeString(date);
		},
		toDateTimeSO: function(time)
		{
			if (time == null) return "";
			var date = new Date(time);				
			return timeStringSO(date);
			function timeStringSO( now ){             
				var ampm= 'am', 
				h= now.getHours(), 
				m= now.getMinutes(), 
				s= now.getSeconds();
				if(h>= 12){
					if(h>12) h -= 12;
					ampm= 'pm';
				}
			
				if(m<10) m= '0'+m;
				var hms = h + ':' + m + ampm;
				return '' + (now.getMonth() + 1) + '/' + now.getDate() + '/' + now.getFullYear() + ' ' + hms;
			}
		},
		count: function(cart) {
  			return cart.items.length;
		},
		countItem: function(cartItems) {
			var itemId = this.linkCtx.elem.getAttribute("item-id");
			var count = 0;
			for (var i =  context.cart.items.length - 1; i >= 0; i--) {
				if (context.cart.items[i].id == itemId) count++;
			};

			//if it's mobile
			return count > 0 ? '<div><span id="item_count">' + count + '</span><span id="inyourcart">in your order</span></div>' : "";
		},
		countItem_listview: function(cartItems) {
			var itemId = this.linkCtx.elem.getAttribute("item-id");
			var count = 0;
			for (var i =  context.cart.items.length - 1; i >= 0; i--) {
				if (context.cart.items[i].id == itemId) count++;
			};
			//if it's listview
			return count > 0 ? '<div><span id="item_count">' + count + '</span></div>' : "";
		},
		selectMenu: function(cartItems) {
			var itemId = this.linkCtx.elem.getAttribute("item-id");
			var count = 0;
			for (var i =  context.cart.items.length - 1; i >= 0; i--) {
				if (context.cart.items[i].id == itemId) count++;
			};

			//return count > 0 ? "list-group-item menuitem active" : "list-group-item menuitem";
			//return count > 0 ? "col-md-4 menu-col-md-4 menuitem active" : "col-md-4 menu-col-md-4 menuitem";

			if (count > 0)
				$(this.linkCtx.elem).addClass("active");
			else
				$(this.linkCtx.elem).removeClass("active");
		},
		plusMinusBtnVisible: function(cartItems) {
			var itemId = this.linkCtx.elem.getAttribute("item-id");
			var count = 0;
			for (var i =  context.cart.items.length - 1; i >= 0; i--) {
				if (context.cart.items[i].id == itemId) {
					count++;
					break;
				}
			};
			return count > 0 ? true : false;
		},
		precise_round: function(subtotal) {
			return precise_round(subtotal, 2);
		},
		currency: function(subtotal) {
			if ($.isNumeric(subtotal))
				return "$ " + precise_round_str(subtotal, 2);
			else
				return subtotal;
		},
		percentage: function(percentage) {
			if ($.isNumeric(percentage))
				return  + percentage + "%";
			else
				return ;
		},
		deliveryTime: function(deliveryTime){
			if (deliveryTime == null)
				return "Select Time";
			else
				return deliveryTime;	
		},
		deliveryDate: function(deliveryDate){
			if (deliveryDate == null)
				return "Select Date";
			else
				return deliveryDate;	
		},
		deliverymethod: function(deliverymethod) {
			if (deliverymethod == null)
				return "Select";
			else if (deliverymethod == 0)
				return "Pickup";
			else if (deliverymethod == 1)
				return "Deliver";
			return "Unknown";				
		},
		disableCss: function(enable) {
			if (enable) {
				return "";
			}
			else {
				return " disabled ";
			}
		},
		openHours: function(openHours) {
			if(!!openHours.str){
				if (openHours && openHours.start == "close") {
					return "closed";
				}
				if(!isJsonString(openHours.str)){
					return "Please Update open hours";
				}
				lArrOpenHours = JSON.parse(openHours.str); // ["11:00-15:00", "18:00-22:00"]
				var timeString = "";
				for (var lTime of lArrOpenHours) {
					//"11:00-15:00"
					timeString += lTime;
					timeString += ", "
				}
				return timeString.substring(0, timeString.length - 2);
			} else {
				if (!openHours) {
					return "closed";
				}
				else {
					if (!openHours.start) {
						return "closed";
					}
					else {
						return openHours.start + ":00 - " + openHours.end + ":00";
					}
				}
			}
		},
		isWeekDay: function(weekDay) {
    		if (weekDay == currentWeekDay()) {
    			return "displayOpenhour";
    		}
		},
		hasPhoneNumber: function(phoneNumber) {
			if(!!phoneNumber) {
				return phoneNumber;
			} else {
				return "No Phone Available"
			}
		},
    	locationUrl: function(address) {
    		// if (context.isMobile) {
    		// 	return "geo:?q="+ address;
    		// }
    		// else {
    		// 	return "http://maps.google.com/?q=" + address;
    		// }

    		// 1/19/2016: this works across the broad, on iOS devices without the google map app, safari opens the google map page
    		return "http://maps.google.com/?q=" + address;
    	},
    	hourNumberToAMPM: function(number) {
    		if ($.isNumeric(number))
    		{
    			if (number <= 11)
    				return number + ' AM';
    			else
    			{
    				if (number >= 13)
    					number -= 12;
    				return number + ' PM';
    			}
    		} else
    			return number;
		},
		rewardsOptionsTable: function(str) {
			return rewardsOptionsTable_converter(str);			
		}
	});
}

function rewardsOptionsTable_converter(str)
{
	var posPts = str.indexOf("pts:");
	if (posPts >= 0)
		return str.substr(posPts + 4);
	else
		return str;
}

function findItemById(itemid) {
	for (var item of context.items)
		if (item.id == itemid)
			return item
}

function newItemSortByItemId(itemids){
	var items = []
	for(var itemid of itemids){
		var item = findItemById(itemid)
		items.push(item);
	}
	items = sortItems(items);

	itemids = [];
	for(var _ of items){
		itemids.push(_.id);
	}
	if (window.cusItemSort) itemids = window.cusItemSort(itemids);
	return itemids;
}

//get menu items in category and save in ccontext.categories
function calcAvailableFilters(context) {
	currentRest.config.dishcategorymapping = currentRest.config.dishcategorymapping || "{}";
	if ((typeof currentRest.config.dishcategorymapping) === "string") // currentRest.config.dishcategorymapping might be already an json, skip converting it if it is already an json
		currentRest.config.dishcategorymapping = JSON.parse(currentRest.config.dishcategorymapping);

    var categories = {};
	var arrReq = [];
	for (var i = 0; i < context.items.length; i++) {
		var menu = context.items[i];			
		if (menu.isHidden)
			continue;		
		if (menuItemShouldStayHidden(menu.item_id))
		{
			if ((menu.isReqOnline && window.isOnline) || (menu.isReqSTO && window.isSTO))
			{
				arrReq.push(menu.id);
			}
			continue;
		}		
		var category = menu.item_id.match(/^[^0-9]+/g);
		var origCategory = menu.item_id.match(/^[^0-9]+/g);
		if (category && category.length && category.length > 0) {
			category = category[0];
		}
		else {
			category = "Unknown";
			origCategory = "Unknown";
		}
		//miyu 4/13/2020 also show in ctgy config added
		if(!!menu.options){
			var menuoptions = JSON.parse(menu.options);
			if (localStorage.selectedAreaItemType) {
				var selectedAreaItemType = JSON.parse(localStorage.selectedAreaItemType);
				var l_selectedAreaItemType = selectedAreaItemType[context.rest.name];
				if (l_selectedAreaItemType) {
					if (menuoptions.areaItemType && menuoptions.areaItemType.indexOf(l_selectedAreaItemType) >= 0) {
						// placeholder
					} else {
						continue;
					}
				}
			}
			if (menuoptions.soldoutForRest && menuoptions.soldoutForRest.indexOf(currentRest.name) >= 0)
				menu.isSoldOut = true;
			// check hideWhenSoldout flag
			if (menu.isSoldOut && (menuoptions.hideWhenSoldout || window.enableHideItemsWhenSoldout))
				continue;
			// check hideWhenNotAvailable flag
			if (menuoptions.hideWhenNotAvailable || window.enableHideItemsWhenNotAvailable) {
				var availableDays = [];
				var availableTime = [];
				if (menuoptions.dishDate) {
					getAvailableDays(menuoptions, availableDays);
				}
				var todayDate = new Date();
				if (menuoptions.dishHour) {
					var todayMillisecond = new Date((todayDate.getMonth() + 1) + '/' + todayDate.getDate() + '/' + todayDate.getFullYear()).getTime();
					getAvailableTime(menuoptions, availableTime, todayMillisecond);
				}
				displayThisDish = checkAvailable(availableDays, availableTime, todayDate.getTime(), todayDate.getDay(), menuoptions);
				if (!displayThisDish)
					continue;
			}
			var alsoshowinCtgyArr = menuoptions.alsoshowinCtgy;
			if (alsoshowinCtgyArr) {
				if (isArrayString(alsoshowinCtgyArr)) {
					alsoshowinCtgyArr = JSON.parse(alsoshowinCtgyArr);
				} else {
					alsoshowinCtgyArr = [alsoshowinCtgyArr];
				}
				//alsoshowinCtgy = alsoshowinCtgy.toLowerCase(); 
				alsoshowinCtgyArr.forEach(function(alsoshowinCtgy) {
				// parse trailing number for sort order (e.g. "饮料03" → category "饮料", sort key "饮料03")
				var _alsoSortMatch = alsoshowinCtgy.match(/^(.+?)(\d+)$/);
				var _alsoSortKey = null;
				if (_alsoSortMatch) {
					_alsoSortKey = alsoshowinCtgy; // full string as sort key e.g. "Printer02"
					alsoshowinCtgy = _alsoSortMatch[1]; // strip number for category name
				}
				var orgalsoshowinCtgy = alsoshowinCtgy;
				//mapping for alsoshowinCtgy name
				if (currentRest.config.dishcategorymapping && currentRest.config.dishcategorymapping[alsoshowinCtgy]){
					if (typeof currentRest.config.dishcategorymapping[alsoshowinCtgy] == 'string')
					{
						alsoshowinCtgy = currentRest.config.dishcategorymapping[alsoshowinCtgy];
					} else
					{
						if (typeof currentRest.config.dishcategorymapping[alsoshowinCtgy] == 'object')
						{
							alsoshowinCtgy = currentRest.config.dishcategorymapping[alsoshowinCtgy][0];
						}
					}
				}
				if (!categories.hasOwnProperty(alsoshowinCtgy)) {
					categories[alsoshowinCtgy] = {
						name : alsoshowinCtgy,
						menus : []
					};
					if (currentRest.config.dishcategorymapping && currentRest.config.dishcategorymapping[orgalsoshowinCtgy]
						&& typeof currentRest.config.dishcategorymapping[orgalsoshowinCtgy] == 'object')
					{
						categories[alsoshowinCtgy].orderIndex = currentRest.config.dishcategorymapping[orgalsoshowinCtgy][1];
					}
					function findOrderIndexByFirstValueOfArray(_dishcategorymapping, firstValueOfArray) {
						if (typeof _dishcategorymapping == 'object') {
							for (var key in _dishcategorymapping) {
								if (_dishcategorymapping.hasOwnProperty(key)) {
									var _array = _dishcategorymapping[key];
									if (Array.isArray(_array) && _array[0] == firstValueOfArray) {
										return _array[1];
									}
								}
							}
						}
					}
					if (currentRest.config.dishcategorymapping) {
						var _index = findOrderIndexByFirstValueOfArray(currentRest.config.dishcategorymapping, orgalsoshowinCtgy);
						if (_index) categories[alsoshowinCtgy].orderIndex = _index;
					}
				}
				categories[alsoshowinCtgy].menus.push(menu.id);
				if (_alsoSortKey) {
					if (!categories[alsoshowinCtgy]._alsoSortKeys) categories[alsoshowinCtgy]._alsoSortKeys = {};
					categories[alsoshowinCtgy]._alsoSortKeys[menu.id] = _alsoSortKey;
				}
			})
			}
		}

		if (currentRest.config.dishcategorymapping)
		{
			if (currentRest.config.dishcategorymapping[category])
			{
				if (typeof currentRest.config.dishcategorymapping[category] == 'string')
				{
					category = currentRest.config.dishcategorymapping[category];
				} else
				{
					if (typeof currentRest.config.dishcategorymapping[category] == 'object')
					{
						category = currentRest.config.dishcategorymapping[category][0];
					}
				}
			}
		}

		if (!categories.hasOwnProperty(category)) {
			categories[category] = {
				name : category,
				menus : []
			};
			if (currentRest.config.dishcategorymapping && currentRest.config.dishcategorymapping[origCategory]
					&& typeof currentRest.config.dishcategorymapping[origCategory] == 'object')
			{
				categories[category].orderIndex = currentRest.config.dishcategorymapping[origCategory][1];
				if (currentRest.config.dishcategorymapping[origCategory][2]) {
					categories[category].description = currentRest.config.dishcategorymapping[origCategory][2];
				}
			}
			// Standalone categoryDescriptions config: { "CategoryName": "description" }
			var _catDescMap = currentRest.config.categoryDescriptions;
			if (_catDescMap) {
				if (typeof _catDescMap === 'string') { try { _catDescMap = JSON.parse(_catDescMap); } catch(e) { _catDescMap = null; } }
				if (_catDescMap && _catDescMap[origCategory]) {
					categories[category].description = _catDescMap[origCategory];
				}
			}
		}
		
		categories[category].menus.push(menu.id);
	};
	// sort categories that have alsoshowin items with sort keys (same expandNumber logic as admin POS)
	var _itemIdMap = {};
	for (var ii = 0; ii < context.items.length; ii++) {
		_itemIdMap[context.items[ii].id] = context.items[ii].item_id;
	}
	function _expandNumber(s) {
		var r = '';
		for (var i = 0; i < s.length; ++i) {
			var j = i;
			if (s[i] >= '0' && s[i] <= '9') {
				for (; s[i] >= '0' && s[i] <= '9' && i < s.length; ++i);
				if ((i - j) < 4) {
					for (var t = 0; t < 4 - (i - j); ++t) r = r + '0';
				}
				for (var t = j; t < i; ++t) r = r + s[t];
				i--;
			} else {
				r = r + s[i];
			}
		}
		return r;
	}
	for (var ctgyKey in categories) {
		if (categories.hasOwnProperty(ctgyKey) && categories[ctgyKey]._alsoSortKeys) {
			var sortKeys = categories[ctgyKey]._alsoSortKeys;
			categories[ctgyKey].menus.sort(function(a, b) {
				var keyA = _expandNumber(sortKeys[a] || _itemIdMap[a] || '');
				var keyB = _expandNumber(sortKeys[b] || _itemIdMap[b] || '');
				if (keyA < keyB) return -1;
				if (keyA > keyB) return 1;
				return 0;
			});
		}
	}
	{
		var mm = Object.keys(categories)[0];;
		categories[mm].menus = categories[mm].menus.concat(arrReq);
	}

    context.categories = [];
    // add stuff the user has ordered before
	var stuffOrdered = {
		name: "Items Ordered Before",
		menus: []
	};
	if (localStorage.orderToGoHistory)
	{
		var orderToGoHistory = JSON.parse(localStorage.orderToGoHistory);
		for (var i = 0; i < orderToGoHistory.length; ++i)
		{
			if (orderToGoHistory[i].order)
			{
				var orderdetails = orderToGoHistory[i].order.orderdetails;
				for (var j = 0; j < orderdetails.length; ++j)
				{
					var item_id = orderdetails[j].item_id;
					if (stuffOrdered.menus.indexOf(item_id) == -1)
						stuffOrdered.menus.push(item_id);
				}
			}
		}
	}
	context.categories.push(stuffOrdered);

	var tempWithoutSort = [];
	var tempWithSort = []
	$.each(categories, function(key, obj){
		if (typeof obj.orderIndex == 'undefined')
			tempWithoutSort.push(obj);
		else
			tempWithSort.push(obj);
	});
	if (tempWithSort.length > 0)
	{
		tempWithSort.sort(function(a, b){
			return a.orderIndex - b.orderIndex;
		});
	}
	context.categories = context.categories.concat(tempWithSort);
	context.categories = context.categories.concat(tempWithoutSort);
	if (window.enableNewSortItem) {
		for (var categorie of context.categories) {
			categorie.menus = newItemSortByItemId(categorie.menus);
		}
	}
	for (var _ of context.categories){
		_.name = localize(_.name);
	}

	// Build upper-category groups from menu items (admin uses this structure)
	// Result shape:
	// context.upperCategories = [ { name: 'zone1', categoryIndexes: [2,5,...] }, ... ]
	if (window.enableUpperCategory) {
		(function buildUpperCategories() {
			try {
				var upperMap = {};
				// categoryIndexes are 1-based in templates (0 is "Items Ordered Before")
				for (var ci = 1; ci < context.categories.length; ci++) {
					var ctg = context.categories[ci];
					var upper = '';
					// find first non-empty upper_category from items under this category
					for (var mi = 0; mi < ctg.menus.length; mi++) {
						var itemId = ctg.menus[mi];
						var item = findItemById(itemId);
						if (item && item.upper_category && String(item.upper_category).trim().length) {
							upper = String(item.upper_category).trim();
							break;
						}
					}
					if (!upper) upper = 'Others';
					if (!upperMap[upper]) {
						upperMap[upper] = { name: upper, categoryIndexes: [] };
					}
					upperMap[upper].categoryIndexes.push(ci);
				}
				// sort groups by the first category index to keep natural order
				var uppers = Object.keys(upperMap).map(function (k) { return upperMap[k]; });
				uppers.sort(function (a, b) {
					// return (a.name < b.name ? -1 : 1);
					return (a.categoryIndexes[0] || 0) - (b.categoryIndexes[0] || 0);
				});
				context.upperCategories = uppers;
			} catch (e) {
				// fail soft — do not block menu rendering
				context.upperCategories = context.upperCategories || [];
			}
		})();
	}
}

function menuItemShouldStayHidden(item_id, db_id)
{
	if (item_id.toLowerCase() == 'hide' || item_id.startsWith('.') || item_id.startsWith('pricedOpt_'))
		return true;

	if (!db_id) return false;
	var menu = context.itemsWithId[db_id];
	if (!!menu && !!menu.options) {
		try {
			var menuoptions = JSON.parse(menu.options);
			if (menuoptions.soldoutForRest && typeof currentRest !== 'undefined' && currentRest && menuoptions.soldoutForRest.indexOf(currentRest.name) >= 0)
				menu.isSoldOut = true;
			// check hideWhenSoldout flag
			if (menu.isSoldOut && (menuoptions.hideWhenSoldout || window.enableHideItemsWhenSoldout))
				return true;
			// check hideWhenNotAvailable flag
			if (menuoptions.hideWhenNotAvailable || window.enableHideItemsWhenNotAvailable) {
				var availableDays = [];
				var availableTime = [];
				if (menuoptions.dishDate) {
					getAvailableDays(menuoptions, availableDays);
				}
				var todayDate = new Date();
				if (menuoptions.dishHour) {
					var todayMillisecond = new Date((todayDate.getMonth() + 1) + '/' + todayDate.getDate() + '/' + todayDate.getFullYear()).getTime();
					getAvailableTime(menuoptions, availableTime, todayMillisecond);
				}
				var displayThisDish = checkAvailable(availableDays, availableTime, todayDate.getTime(), todayDate.getDay(), menuoptions);
				if (!displayThisDish)
					return true;
			}
		} catch (e) {
			console.error('Error parsing menu options for item ' + db_id + ':', e);
		}
	}

	return false;
}

function precise_round_str(num, decimals) {
	var sign = num >= 0 ? 1 : -1;
    return (Math.round((num*Math.pow(10,decimals))+(sign*0.001))/Math.pow(10,decimals)).toFixed(decimals);
}

function precise_round(num, decimals) {    
    return parseFloat( precise_round_str(num, decimals) );
}

function timeString( now ){             
    var ampm= 'am', 
    h= now.getHours(), 
    m= now.getMinutes(), 
    s= now.getSeconds();
    if(h>= 12){
        if(h>12) h -= 12;
        ampm= 'pm';
    }

    if(m<10) m= '0'+m;
    var hms = h + ':' + m + ampm;
    return '' + (now.getMonth() + 1) + '/' + now.getDate() + '/' + now.getFullYear() + ' ' + hms;
}

function renderMenuPage(rest) {
	throw new Exception('NotImplemented');
}

function getWeekDay(offset) { 
    var days = ['sun','mon','tue','wed','thu','fri','sat'] 
    return days[offset];
}

function isJsonString(str) {
    try {
        JSON.parse(str);
    } catch (e) {
        return false;
    }
    return true;
}

function isArrayString(str) {
	try {
		var arr = JSON.parse(str);
		if (Array.isArray(arr)) {
			return true;
		}
	} catch (e) {}
	return false;
}

// This should be a convertor.
function inOpenHours(rest)
{
	if (!rest.config || !rest.config.openHours) {
		return false;
	}
	
	var openHours = rest.config.openHours[currentWeekDay()];
	if (openHours.start == "close") {
		return false;
	}
	if (!openHours.str) {
		return true;
	}
	var begin = [];
	var end = [];
	
	if(!isJsonString(openHours.str)){
		return false;
	}

	var openTimeRange = JSON.parse(openHours.str);		
	for (var i = 0; i < openTimeRange.length; i++) 
	{
		var timePeriod = openTimeRange[i].split("-");
		begin[i] = timePeriod[0].toString().padStart(2, '0');
		end[i] = timePeriod[1].toString().padStart(2, '0');
	}
	if (begin[0] >= end[end.length - 1]) 
	{
		end = null;
	}
	var curTime = new Date();
	var curHour = curTime.getHours().toString().padStart(2, '0');
	var curMinute = curTime.getMinutes().toString().padStart(2, '0');
	var endHour;
	var endMinute;
	if (end && end.length) 
	{
		endHour = end[end.length - 1].split(":")[0];
		endMinute = end[end.length - 1].split(":")[1];
	}
	if (end && (curHour > endHour || (curHour == endHour && curMinute >= endMinute))) 
	{
		return false;
	}
	return true;
}

function loadRestaurantsData(callback) {
	if (restaurants) {
		// performance improvement if restaurants have been loaded before.
		callback(restaurants);
		return;
	}

	var url = '/m/api/restaurants/filter/togo';
	if (window._pageName == "index_new_mobile" || window._pageName == "index_mobile_dinein" || window._pageName == "index_track" || window._pageName == "STO Pay" || window._pageName == "index_tabletorder")
		url = '/m/api/restaurants/filter/sto';
	$.get(url)
    .done(function(data) {
		data.sort(function(a, b){
			var aa = parseFloat(a.orderTogoOrderWeight || 0);
			var bb = parseFloat(b.orderTogoOrderWeight || 0);
			return aa - bb;
		}); 
		restaurants = data;
	    callback(data);
	});
}

function sortAndMergeMenuItems(obj)
{
	var idToMenuItem = {};
	for (var i = 0; i < obj.items.length; ++i)
	{
		var item = JSON.parse(JSON.stringify(obj.items[i])); // need to make a copy here
		var key = item.id;
		if(!!item.optionsstr && item.optionsstr != ""){
			//no priced item
			key += ":" + item.optionsstr;
		}
		if(item.optionitemids && item.optionitemids.length > 0){
			key += ":" + JSON.stringify(item.optionitemids);
		} 
		if(item.optionitemobjects && item.optionitemobjects.length > 0){
			key += ":" + JSON.stringify(item.optionitemobjects);
		} 
		if(!!item.specialIns && item.specialIns != ""){
			key += ":" + item.specialIns;
		} 
		if(!!item.togo && item.togo != ""){
			key += ":" + item.togo;
		} 
		if(!!item.price && item.price != ""){
			key += ":" + item.price;
		} 
		if (window.isSTO && item.letterId && !window.disableSharedCart) {
			key += ":" + item.letterId;
		}
		if (item.id == -2){   //for rewards value Redeem, need to separate rewards value item based on the price
			//some of old item might not have rewardsInfo...like selforder order detials.
			if(!!item.rewardsInfo){
				key += ":" + item.price + ":" + item.rewardsInfo.item2reward;
			} else {
				key += ":" + item.price + ":";
			}
		} 
			
		if (idToMenuItem[key] == null)
		{
			//new Item
			idToMenuItem[key] = item;
			idToMenuItem[key].qty = 1;
			idToMenuItem[key].index = i;
			if(item.optionsstr){
				idToMenuItem[key].optionsstr = item.optionsstr;
			}
			if(item.optionitemids){
				idToMenuItem[key].optionitemids = item.optionitemids;
				idToMenuItem[key].optionitemobjects = item.optionitemobjects;
			}
			if(item.optionitemobjects){
				idToMenuItem[key].optionitemobjects = item.optionitemobjects;
			}
			if(item.specialIns){
				idToMenuItem[key].specialIns = item.specialIns;
			}
			if(item.togo){
				idToMenuItem[key].togo = item.togo;
			}
			idToMenuItem[key].cartIndex = [];
			idToMenuItem[key].cartIndex.push(i);
		} else
		{
			//same Item
			idToMenuItem[key].qty++;					// as we'll do in place modify here
			idToMenuItem[key].price += item.price;
			if (item.onlineFee) idToMenuItem[key].onlineFee = precise_round((idToMenuItem[key].onlineFee || 0) + item.onlineFee, 2);
			idToMenuItem[key].cartIndex.push(i);
		}
	}
	obj.sortedItems = [];
	for (var key in idToMenuItem)
		obj.sortedItems.push(idToMenuItem[key]);

	obj.sortedItems.sort(function(a, b){
		if (a.id > b.id)
		    return 1;
		else if (a.id < b.id)
		    return -1;
		else 
		    return 0;
	})
}

function clearCart() {
	context.cart = getEmptyCart();
	saveCart(currentRest, context.cart);
}

function getEmptyCart () {
	return { subtotal: 0.0, items: [], tip: -1.0, sortedItems: [] };
}

//only used in home.base.js
function openPopup() {
	setTimeout(function () {
		console.log("pop up ");
		$('#trackOrder-pop-up').css('display','block');
	}, 500);
};

function getCart(rest) {
	if (window.isSTO && !window.disableSharedCart) {
		// get shared cart instead
		var params = new URLSearchParams(window.location.search);
		var showHistory = params.get("showHistory");
		if (!showHistory) {
			getSharedCart(true);
		} else {
			getSharedCart(false, function() {
				$('.history-button-history').click();
				setTimeout(function(){
					// for msg1: "You have unsubmitted items in your cart.",
					// remove showHistory from url
					params.delete("showHistory");
					var newUrl = window.location.protocol + "//" + window.location.host + window.location.pathname;
					var paramStr = params.toString();
					if (paramStr && paramStr.length > 0) {
						newUrl += "?" + paramStr;
					}
					window.history.replaceState({}, document.title, newUrl);
				}, 1200);
			});
		}
		return getEmptyCart();
	}

	var cartKey = "order.rest" + rest.name + rest.id;
	if (!localStorage[cartKey]) {
		return getEmptyCart();
	} else {
		if (JSON.parse(localStorage[cartKey]).sortedItems) {
			return JSON.parse(localStorage[cartKey]);
		} else {
			return getEmptyCart();
		}
	}
}

function saveCart(rest, cart) {
	var cartKey = "order.rest" + rest.name + rest.id;
	localStorage[cartKey] = JSON.stringify(cart);
}


function showRestaurantOpenSchedule()
{
	var container = $('#modalConfirm');
	var ctx = {
		title: "Business Hours",
		openHours: currentRest.config.openHours
	};
	$.templates.restaurantClosed.link(container, ctx);
	container.modal('show');
	//for highlighting day
	var date = new Date();
	var day = date.getDay();
	if(day == 0){
		var tr = "#hoursTable tr:eq(7)";
	} else {
		var tr = "#hoursTable tr:eq(" + day + ")";
	}
	$(tr).addClass("highlight");
}

function showCustomAlert(data)
{
	var ctx = {
		alerttype: data.alerttype,
		title : data.title,
		msg1 : data.msg1,
		msg2 : data.msg2,
		ok_msg: data.ok_msg,
		cancel_msg: data.cancel_msg,
		ok_func: data.ok_func,
		cancel_func: data.cancel_func,
		cart_msg: data.cart_msg,
		cart_func: data.cart_func
	}
	var container = $('#modalConfirm');
	$.templates.customAlert.link(container, ctx);
	container.modal('show');
	$('#modalConfirm').css("z-index","1053");   //so that it goes above the drop 	
	$('.modal-backdrop.in').css("z-index","1052");	

	if (ctx.ok_func)
	{
		$('#customAlert-OK').unbind().tap( function(){
			ctx.ok_func(container);
		});
	} else
	{
		$('#customAlert-OK').unbind().tap( function(){
			container.modal('hide'); 
			if (window._S300IPs && typeof g_cid != 'undefined' && window._S300IPs[g_cid] && data.title == "Authorization failed") {
				$("#so-backToMenu-btn").trigger("click");
			}
		});
	}
	if (ctx.cancel_func)
	{
		$('#customAlert-cancel').unbind().tap( function(){
			ctx.cancel_func(container);
		});
	}
	if (ctx.cart_func)
	{
		$('#customAlert-cart').unbind().tap( function(){
			ctx.cart_func(container);
		});
	}
}

function showCustomOverAmountSignAlert(data)
{
	var ctx = {
		alerttype: data.alerttype,
		title : data.title,
		msg1 : data.msg1,
		msg2 : data.msg2,
	}
	var container = $('#modalConfirm');
	$.templates.customOverAmountSignAlert.link(container, ctx);
	container.modal('show');
	$('#modalConfirm').css("z-index","1053");   //so that it goes above the drop 	
	$('.modal-backdrop.in').css("z-index","1052");	
}

function displayCustomTipAlert(init)
{
	var ctx = {
		title: "Enter Tip",
	}
	var container = $('#modalConfirm');
	$.templates.customTipAlert.link(container, ctx);
	container.modal('show');
	$('.customTips').removeClass('open');
	$('#modalConfirm').css("z-index","1053");   //so that it goes above the drop 	
	$('.modal-backdrop.in').css("z-index","1052");	
	$(".dropdown-backdrop").remove();
	$('#tip-input').focus();
	var input = '';
	init = init || '0.00';
	if(init.includes('%')) {
		init = init.replace('%', '') / 100;
		init = (init * context.cart.subtotal).toFixed(2);
		input = init.replace('.', '').replace(/^0+/, '');
	}
	else if(init == '0.00')
		input = '';
	else
		input = init.replace('.', '').replace(/^0+/, '');
	$('#tip-input').val(init);
	var tip = init;
	$('#tip-input').keydown(function(event) {
		if(input.length == 0) {
			// 1-9 only
			if(event.keyCode <= 48 || event.keyCode > 57 || isNaN(event.key))
				return false;
			else
				input+=event.key;
		}
		else {
			// 0-9 only
			if(event.keyCode == 8)
				input = input.slice(0, -1);
			else if(event.keyCode < 48 || event.keyCode > 57 || isNaN(event.key))
				return false;
			else
				input+=event.key;
		}
		// modify tip
		if(input.length > 2)
			tip = input.substr(0, input.length - 2) + '.' + input.substr(input.length - 2);
		else if(input.length == 2)
			tip = '0.' + input;
		else if(input.length == 1)
			tip = '0.0' + input;
		else
			tip = '0.00';
		$('#tip-input').val(tip);
		event.preventDefault();
	});
	$('#customAlert-apply').unbind().tap( function(){
		container.modal('hide');
		document.activeElement.blur();
		$("#customTipBtn").text("Custom ");
		$("#customTipBtn").append('<span class="caret" style="margin-left: 5px;"></span>');
		$("#customTipBtn").addClass("tip-active");
		$(".btn-tip-percent").removeClass("tip-active");
		var tipPercent = tip / context.cart.subtotal;
		context.cart.tipPercent = tipPercent.toFixed(2);
		$.observable(context.cart).setProperty("tip", precise_round(context.cart.subtotal * (+tipPercent), 2));
	});
	$('#customAlert-cancel').unbind().tap( function(){
		container.modal('hide');
		document.activeElement.blur();
	});
	$('#alertClose').unbind().tap( function(){
		container.modal('hide');
		document.activeElement.blur();
	});
}

//used in renderMenuPage
function getCustomerName() {
	if (!localStorage["customername"]) {
		return "";
	} else {
		return localStorage["customername"];
	}
}

function getCustomerPhone() {
	if (!localStorage["customerphone"]) {
		return "";
	} else {
		return localStorage["customerphone"];
	}
}

function getAvailableDays(optsStrings, availableDays){
	var dishDateArr = optsStrings.dishDate.split(',');
	if (dishDateArr.length)
	{
		for (var i = 0; i < dishDateArr.length; i++)
		{
			var dateRangeArr = dishDateArr[i].split('-');
			if (dateRangeArr.length > 1)
			{
				var start = parseInt(dateRangeArr[0]);
				var end = parseInt(dateRangeArr[1]);
				if (start <= end)
				{
					for (var j = start; j <= end; j++)
					{
						availableDays.push(j);
					}
				}
			} else
			{
				if (!isNaN(dishDateArr[i]))
				{
					availableDays.push(parseInt(dishDateArr[i]))
				}
			}
		}
	}
}

function getAvailableTime(optsStrings, availableTime, todayMillisecond){
	var dishHourArr = optsStrings.dishHour.split(',');
	if (dishHourArr.length)
	{
		for (var i = 0; i < dishHourArr.length; i++)
		{
			var timeRangeArr = dishHourArr[i].split('-');
			if (timeRangeArr.length > 1)
			{
				var startHourMinuteArr = timeRangeArr[0].split(':');
				var startHour = startHourMinuteArr[0];
				var startMinute = 0;
				if (startHourMinuteArr.length > 1)
				{
					startMinute = startHourMinuteArr[1];
				}

				var endHourMinuteArr = timeRangeArr[1].split(':');
				var endHour = endHourMinuteArr[0];
				var endMinute = 0;
				if (endHourMinuteArr.length > 1)
				{
					endMinute = endHourMinuteArr[1];
				}

				var startTime = todayMillisecond + parseInt(startHour * 3600000 + startMinute * 60000);
				var endTime = todayMillisecond + parseInt(endHour * 3600000 + endMinute * 60000);
				startTime += (((new Date(startTime).getTimezoneOffset()) - (new Date(todayMillisecond).getTimezoneOffset())) * 60 * 1000);
				endTime += (((new Date(endTime).getTimezoneOffset()) - (new Date(todayMillisecond).getTimezoneOffset())) * 60 * 1000);
				availableTime.push([startTime, endTime]);
			}
		}
	}
}

function checkAvailable(availableDays, availableTime, currentTime, dayOfTheWeek, options, todayMillisecond){
	if (options.dishHourDate) {
		var dishHourDateArr = options.dishHourDate.split(/\s*,\s*/g);
		var _todayMs = todayMillisecond || new Date((new Date().getMonth() + 1) + '/' + new Date().getDate() + '/' + new Date().getFullYear()).getTime();
		for (var _ of dishHourDateArr) {
			var hour_date_arr = _.split(";");
			var _availableDays = [];
			var _availableTime = [];
			getAvailableDays({ dishDate: hour_date_arr[1] }, _availableDays);
			getAvailableTime({ dishHour: hour_date_arr[0] }, _availableTime, _todayMs);
			if (_availableDays.indexOf(dayOfTheWeek) != -1 && _availableTime.length > 0 && _availableTime[0][0] <= currentTime && _availableTime[0][1] >= currentTime) {
				return true;
			}
		}
		return false;
	}

	if (availableDays.length > 0)
	{
		if (availableDays.indexOf(dayOfTheWeek) == -1)
		{
			return false;
		}
	}

	if (availableTime.length > 0)
	{
		for (var i = 0; i < availableTime.length; i++)
		{
			if (availableTime[i][0] <= currentTime && availableTime[i][1] >= currentTime)
			{
				return true;
			}
		}
		return false;
	}
	return true;
}

// 9/17/2019 Miyu: udpated with new param isUpdate
// if it is update, need to exclude the count in current cart that is going to be updated
function checkRestricted(maxQuantRestriction, id, numNewItem, isUpdate) {
	var items = context.cart.items;
	var count = 0;
	if (numNewItem > maxQuantRestriction)
	{
		return true;
	}
	for (var i = 0; i < items.length; i++)
	{
		if (id == items[i].id)
		{
			count++;
		}
		if(isUpdate){
			if ((numNewItem - count) > maxQuantRestriction)
			{
				return true;
			}
		} else {
			if (count + numNewItem > maxQuantRestriction)
			{
				return true;
			}
		}
	}
	return false;
}

function displayUnavailableWarning(dishDate, dishHour, dishHourDate){
	var message = 'This dish is unavailable currently, it can only be ordered';
	if (dishHourDate) message = 'This dish is unavailable currently. ';
	var dishDateArr = dishDate.split(',');
	var dishHourArr = dishHour.split(',');
	var firstOn = true;
	if (dishDateArr.length > 0)
	{
		for (var i = 0; i < dishDateArr.length; i++)
		{
			var dishDateRangeArr = dishDateArr[i].split('-');
			if (dishDateRangeArr.length > 1)
			{
				var startDay = parseInt(dishDateRangeArr[0]);
				var endDay = parseInt(dishDateRangeArr[1]);
				if (startDay <= endDay)
				{
					if (firstOn)
					{
						message += ' on';
						firstOn = false;
					}
					for (var j = startDay; j <= endDay; j++)
					{
						message += ' ' + number2Day[j] + ',';
					}
				}
			} else
			{
				if (!isNaN(parseInt(dishDateRangeArr[0])))
				{
					if (firstOn)
					{
						message += ' on';
						firstOn = false;
					}
					message += ' ' + number2Day[parseInt(dishDateRangeArr[0])] + ',';
				}
			}
		}
	}
	if (dishHourArr.length > 0)
	{
		var dishHourRangeArr = dishHourArr[0].split('-');
		if (dishHourRangeArr.length > 1)
		{
			message += ' during';
			for (var i = 0; i < dishHourArr.length; i++)
			{
				message += ' ' + dishHourArr[i] + ',';
			}
		}
	}
	message = message.substr(0, message.length - 1);
	var data = {
		alerttype: "",
		title: "Dish Unavailable",
		msg1 : message,
		msg2 : ''
	}
	showCustomAlert(data);
}

function displaySoldOutWarning(){
	var data = {
		alerttype: "Info: ",
		title : "Dish Unavailable",
		msg1 : "This item was sold out.",
		msg2 : 'Please select other dishes'
	};
	if(!location.href.includes('tabletorder')) {
		showCustomAlert(data);
	}

	// MERGE
	// $("#modalWarning #optLabel").html('Dish unavailable');
	// $('#unavailableMsg').html('This item was sold out.');
	// $('#modalWarning').modal('show');
}

function displayRestrictedWarning(maxQuantRestriction) {
	var message = "This item is restricted to be ordered only " + maxQuantRestriction + " for each order.";
	$('#modalWarning #optLabel').html('Dish unavailable');
	$('#unavailableMsg').html(message);
	$('#modalWarning').modal('show');
}

function cartClearAllClicked(){
	cartClearAll();
	checkCartHasItems();
	showEmptyCartMsg();
}

function cartClearAll(){
	for(var i = 0; i < context.cart.items.length; ++i){
		menuItemDiv = $(".menuitem[item-id='" + context.cart.items[i].id + "']");
		manuallyUpdateUIForCart_clearAll($(menuItemDiv));
	}

	//clear items in cart
	context.cart.items.splice(0,context.cart.items.length);
	context.cart.sortedItems.splice(0, context.cart.sortedItems.length);
	context.cart.subtotal = 0;
	saveCart(currentRest, context.cart);
}

//
function increaseItemCount(itemCount){
	
	itemCount++;
	return itemCount;
}
//
function decreaseItemCount(itemCount){
	itemCount--;
	return itemCount;
}

//update item total price by multiping the numbers of item
function updateItemsTotalPriceInDetails(newCount, item_price) {
	var newTotalPrice;
	if(newCount > 0){
		newTotalPrice = item_price * newCount;	
	} else if (newCount == 0){
		newTotalPrice = 0;
	} else {
		console.log("invalid item count!");
	}
	$("#item_details_page .item_price_final").html("<a>$" + precise_round(newTotalPrice, 2).toFixed(2) + "</a>");
}


// common code for order.base.mobile.js & selforder.base.js
//display grouped option minium msg
function appendGroupedMinMsg(container, groupedChoiceList){
    if(!!groupedChoiceList && Object.keys(groupedChoiceList).length > 0){
        for(ctgy in groupedChoiceList){
            var groupminMsgSpan = $('<span class="groupminMsg">');
            var ctgyname = groupedChoiceList[ctgy].ctgyname;
            var uniqueNames = [];
            $.each(ctgyname, function(i, el){
                if($.inArray(el, uniqueNames) === -1) uniqueNames.push(el);
            });
            var groupname =  uniqueNames.toString();
            var message = "* Choose at most " + groupedChoiceList[ctgy].maxChoice + " options from Group" + groupname.toUpperCase();
            groupminMsgSpan.append(message);
            groupminMsgSpan.append('<br>');
            container.append(groupminMsgSpan);
        }
    }
}

function appendGroupedChoiceAttr(table, ctgyname, groupedChoiceList){
    var g_ctgyname = ctgyname.charAt(ctgyname.lastIndexOf("-") + 1);
    var groupedChoice = groupedChoiceList[g_ctgyname];
    if(!!groupedChoice){
        table.attr('ctgyGroup', groupedChoice.group);
        table.attr('ctgyGroupMax', groupedChoice.maxChoice);
    }
}


function addToGroupedChoiceList(groupedChoiceList, ctgyname){  
	var lastHyphenSignIndex = ctgyname.lastIndexOf("-");
	var charAtLastHyphenSignIndexPlusOne = ctgyname.charAt(lastHyphenSignIndex + 1);
	var charAtLastHyphenSignIndexPlusTwo = ctgyname.charAt(lastHyphenSignIndex + 2);
	if (lastHyphenSignIndex != -1 && charAtLastHyphenSignIndexPlusOne.match(/[a-zA-Z]/) != null && !isNaN(parseInt(charAtLastHyphenSignIndexPlusTwo))) 
    {
		var groupedChoiceName = charAtLastHyphenSignIndexPlusOne;
		var maxNumChoice = parseInt(charAtLastHyphenSignIndexPlusTwo);
		if(!groupedChoiceList.hasOwnProperty(groupedChoiceName)){
			groupedChoiceList[groupedChoiceName] = {
				group: groupedChoiceName,
				maxChoice: maxNumChoice,
				count: 0,
				ctgyname: []
			};
		}
		groupedChoiceList[groupedChoiceName].ctgyname.push(charAtLastHyphenSignIndexPlusOne);
    }
    
} 

function displayGroupedMinWarning(groupname, max) {
	var message = "You can only choose at most " + max + " options from " + groupname;
	var data = {
		alerttype: "Error: ",
		title: "Max # options restriction",
		msg1 : message,
		msg2 : 'Please reduce the number of options to meet the requirement'
	}
	showCustomAlert(data);
}

function getCtgynameString(meetGroupedMinium){
    var ctgynames = ""; 
    for(var i = 0; i < meetGroupedMinium.selectedTrs.length; i++){	
        var optionName = $(meetGroupedMinium.selectedTrs[i]).closest("table").attr('option_name');	
        ctgynames += optionName.substring(0, optionName.lastIndexOf("-"));
        if(i != meetGroupedMinium.selectedTrs.length - 1){
            ctgynames += ", ";
        }
    }
    return ctgynames;
}

//check if selected options meets the grouped choice limitation
function checkGroupedChoice(groupedChoiceList, container){
	var result = {
		withinMax: true, 
		selectedTrs: null,
		max: 0,
		groupedItemsRequiredCheck: {},
	};
	for(ctgy in groupedChoiceList){
		groupedChoiceList[ctgy].count = 0;   //reset count 
	}
	$(".uv_optionsTable-holder table").each(function(){
		var option_name = $(this).attr('option_name');
		var thisCtgyGroup = $(this).attr('ctgyGroup');
		var ctgyGroupMax = $(this).attr('ctgyGroupMax');
		var stepnum = $(this).attr('stepnum');
	
		if (stepnum != undefined && $("#addToGlobalCart").attr("cur_stepnum") != undefined && $("#addToGlobalCart").attr("cur_stepnum") != stepnum) {
			return true;
		}
		if(!!thisCtgyGroup && !!ctgyGroupMax && groupedChoiceList.hasOwnProperty(thisCtgyGroup)){
			var table_type = $(this).attr('table_type');
			var selected = $('.uv_optionsTable-holder table[option_name="' + option_name + '"][table_type="' + table_type + '"] input:checked');
			if(!!selected && selected.length > 0){
				var selectedCount = 0;
				if(table_type == "checkbox"){
					selectedCount = getSelectedOptionCount(selected);
					if(selectedCount == 0){
						selectedCount = selected.length;
					}
				} else {
					selectedCount = selected.length;   //if it is "radio"  nth to do with counter
				}
				groupedChoiceList[thisCtgyGroup].count += selectedCount;
				if(groupedChoiceList[thisCtgyGroup].count > groupedChoiceList[thisCtgyGroup].maxChoice){
					var selectedTrs = $('.uv_optionsTable-holder table[ctgyGroup="' + thisCtgyGroup + '"] thead tr');
					HighlightGroupTable(thisCtgyGroup, container);
					result.withinMax = false;
					result.selectedTrs = selectedTrs;
					result.max = ctgyGroupMax;
				}	
				result.groupedItemsRequiredCheck[thisCtgyGroup] = {
					selectedCount: groupedChoiceList[thisCtgyGroup].count,
					max: groupedChoiceList[thisCtgyGroup].maxChoice,
				}
			}
		}
		if (stepnum != undefined && $("#addToGlobalCart").attr("cur_stepnum") != undefined && $("#addToGlobalCart").attr("cur_stepnum") == stepnum) {
			// return false;
		}
	});
	return result;
}

// count total selected option count of the same ctgy and return the totalCount in order to check Max & min
function getSelectedOptionCount(selectedOptions){
	var totalCount = 0;
	for(var i = 0; i < selectedOptions.length; i++){
		var optionsCount = Number($(selectedOptions[i]).attr("count"));
		if(!!optionsCount){
			totalCount += optionsCount;
		}	
	}
	return totalCount;
}

function HighlightGroupTable(thisCtgyGroup, container){
	var selectedTrs = $('.uv_optionsTable-holder table[ctgyGroup="' + thisCtgyGroup + '"] thead tr');
	
	//need to scroll to top before frash
	if(!isScrolledIntoView(selectedTrs, container)){
		var elemTop = $(selectedTrs).offset().top;
		var containerOffset = $('.uv_optionsTable-holder').offset().top;
		var scrollposition = Math.abs(containerOffset -  elemTop);
		if(scrollposition > 10){
			scrollposition -= 10;
		}
		container.animate({scrollTop: scrollposition}, 300, 'swing');

		setTimeout(function(){
			selectedTrs.addClass("highlightClass");
		}, 400);
	} else {
		selectedTrs.addClass("highlightClass");
	}

	setTimeout(function(){
		selectedTrs.removeClass("highlightClass");
		selectedTrs.focus();
	}, 800);
}

function updateItemsPriceWithOptions(item_price, optionPriceList){
	var updatedPrice = item_price;
	for(option_name in optionPriceList){
		updatedPrice += parseFloat(optionPriceList[option_name].price);
	}
	return updatedPrice;
}

//look for the item in cart, if not found return 0
function getItemCount(menuItem, cart_itemList){
	var menuList = cart_itemList.items;
	var count = 0;
	for(var i = 0 ; i < menuList.length ; i++){
		if(menuItem.id == menuList[i].id){
			count++;
		}
	}
	return count;
}

//05/01/2018 miyu: add item to cart with options 
function addItemWithOptionToCart(index, selectedOptions, numNewItem, maxQuantRestriction, weight){

	var item = context.items[index];

	item.id = context.items[index].id;
	item.item_id = context.items[index].item_id;
	item.name = context.items[index].name;
	item.optionAddedPrice = context.items[index].price + selectedOptions.optiontotalprice;
	if (weight) {
		var weightPrice = weight * parseFloat(JSON.parse(item.options).pricePerUnit);
		if (!isNaN(weightPrice)) {
			item.weight = weight;
			selectedOptions.optionsstr += weight + " lb. $" + JSON.parse(item.options).pricePerUnit + " per lb."
			item.optionAddedPrice += weightPrice;
		}
	}
	item.optionsstr = selectedOptions.optionsstr;
	item.optionitemids = selectedOptions.optionitemids;
	item.optionitemobjects = selectedOptions.optionitemobjects;
	item.specialIns = selectedOptions.specialIns;
	item.togo = selectedOptions.togo;
	item.taxrate = context.items[index].taxrate;
	if (window.adjustPriceCB) {
		item.optionAddedPrice = window.adjustPriceCB(item);
	}

	//dumplicate item by the itemCount 
	for(var i = 0; i < numNewItem ; i++){
		var itemToAdd = {
			id: item.id, 
			item_id: item.item_id, 
			name: item.name, 
			price: item.optionAddedPrice, 
			optionsstr: item.optionsstr, 
			optionitemids: item.optionitemids, 
			optionitemobjects: item.optionitemobjects, 
			specialIns: item.specialIns, 
			taxrate: item.taxrate, 
			itemIndex: index, 
			optionPriceList: selectedOptions.optionPriceList, 
			togo: item.togo,
			weight: item.weight,
			onlineFee: item.onlineFee,
		}
		if (window.isSTO && !window.disableSharedCart) itemToAdd.uuid = generateUUID();
		if (maxQuantRestriction > 0)
		{
			itemToAdd.maxQuantRestriction = maxQuantRestriction;
			context.cart.items.push(itemToAdd);
		} else
		{
			context.cart.items.push(itemToAdd);
		}		
		context.cart.subtotal = context.cart.subtotal + item.optionAddedPrice;
	}
	
	var menuitem = $('.menuitem[id="' + index + '"]');
	//update the UI in menu
	while (menuitem && !$(menuitem).hasClass("menuitem"))
		menuitem = menuitem.parentElement;
	manuallyUpdateUIForCart($(menuitem));

	onMenuLoaded(true);

	saveCart(currentRest, context.cart);
	return false;
}

//05/01/2018 miyu: update item to cart with options 
function updateItemWithOptionToCart(sortedCartIndex, index, selectedOptions, numNewItem, maxQuantRestriction, weight){

	var item = context.items[index];

	item.id = context.items[index].id;
	item.item_id = context.items[index].item_id;
	item.name = context.items[index].name;
	item.optionAddedPrice = context.items[index].price + selectedOptions.optiontotalprice;
	if (weight) {
		var weightPrice = weight * parseFloat(JSON.parse(item.options).pricePerUnit);
		if (!isNaN(weightPrice)) {
			item.weight = weight;
			selectedOptions.optionsstr += weight + " lb. $" + JSON.parse(item.options).pricePerUnit + " per lb."
			item.optionAddedPrice += weightPrice;
		}
	}
	item.optionsstr = selectedOptions.optionsstr;
	item.optionitemids = selectedOptions.optionitemids;
	item.optionitemobjects = selectedOptions.optionitemobjects;
	item.specialIns = selectedOptions.specialIns;
	item.togo = selectedOptions.togo;
	item.taxrate = context.items[index].taxrate;

	//remove old item
	//var oldItem = context.cart.items[index];
	//need to find item from cart.items 
	var itemToDelete = context.cart.sortedItems[sortedCartIndex];
	var itemToDelete_qty = itemToDelete.qty;
	while(itemToDelete_qty > 0){
		var cartIndexes = itemToDelete.cartIndex;  //[]
		context.cart.items.splice(cartIndexes.pop(), 1);
		itemToDelete_qty--;		
	}
	context.cart.subtotal = context.cart.subtotal - itemToDelete.price ;
	
	if (window.adjustPriceCB) {
		item.optionAddedPrice = window.adjustPriceCB(item);
	}
	//dumplicate item by the itemCount 
	for(var i = 0; i < numNewItem ; i++){
		var itemToAdd = {
			id: item.id, 
			item_id: item.item_id, 
			name: item.name, 
			price: item.optionAddedPrice, 
			optionsstr: item.optionsstr, 
			optionitemids: item.optionitemids, 
			optionitemobjects: item.optionitemobjects, 
			specialIns: item.specialIns, 
			taxrate: item.taxrate, 
			itemIndex: index, 
			optionPriceList: selectedOptions.optionPriceList, 
			togo: item.togo,
			weight: item.weight,
			onlineFee: item.onlineFee,
		}
		if (window.isSTO && !window.disableSharedCart) itemToAdd.uuid = generateUUID();
		if (maxQuantRestriction > 0)
		{
			itemToAdd.maxQuantRestriction = maxQuantRestriction;
			context.cart.items.push(itemToAdd);
		} else
		{
			context.cart.items.push(itemToAdd);
		}		
		context.cart.subtotal = context.cart.subtotal + item.optionAddedPrice ;
	}
	
	var menuitem = $('.menuitem[id="' + index + '"]');
	//update the UI in menu
	while (menuitem && !$(menuitem).hasClass("menuitem"))
		menuitem = menuitem.parentElement;
	manuallyUpdateUIForCart($(menuitem));
	onMenuLoaded(true);
	saveCart(currentRest, context.cart);
	viewCartClicked();  //reopen cart for dinein & mesh 
	return false;
}


function showEmptyCartMsg(){
	var readyOrderBtn = $("#readyOrder");
	if (readyOrderBtn[0]) readyOrderBtn[0].innerHTML = "Keep Browsing";
	readyOrderBtn.addClass("keepBrowsingBtn");
	$(".emptyCartMsg").addClass("show");
}

//dinein, togo, mesh
function manuallyUpdateUIForCart(obj, deleteItem, skipSharedCartUpdate)
{
	if (obj.length > 0)
	{
		// old overlay logic --- 
		// 
		// var overlayMenuItem = obj.find(".overlayMenuItem")[0];
		// var innerHTML = $.views.converters.countItem.apply( {linkCtx: {elem: overlayMenuItem}} );
		// overlayMenuItem.innerHTML = innerHTML;	
		// // Works perfectly fine in Chrome, but had to add this following code because of a stupid Safari bug
		// // had to manually set the visibility of overlayMenuItem, otherwise it won't update in Safari, despite the fact that I have the same css rule
		// if (innerHTML != '')
		// {
		// 	overlayMenuItem.style.display = '';
		// } else
		// {
		// 	overlayMenuItem.style.display = 'none';
		// }

		// 04/13/2020 now the overlay Menu Items might be multiple, since we have also show in ctgy. 
		var overlayMenuItems = $('.overlayMenuItem[item-id="' + obj.attr("item-id") + '"]');
		for(var i = 0; i < overlayMenuItems.length ; i++){
			//need to go increate overlay Counter for each item found with attribute based on item-id
			var overlayMenuItem = $(overlayMenuItems[i])[0];
			var innerHTML = $.views.converters.countItem.apply( {linkCtx: {elem: overlayMenuItem}} );
			overlayMenuItem.innerHTML = innerHTML;
			
			// Works perfectly fine in Chrome, but had to add this following code because of a stupid Safari bug
			// had to manually set the visibility of overlayMenuItem, otherwise it won't update in Safari, despite the fact that I have the same css rule
			if (innerHTML != '')
			{
				overlayMenuItem.style.display = '';
			} else
			{
				overlayMenuItem.style.display = 'none';
			}
		}
	}
	var numItemsSpans = $("#numItems").find("span");
	if (numItemsSpans.length > 0)
	{
		numItemsSpans[0].innerHTML = context.cart.items.length;
	}
	var orderDineinBtn = $("#readyOrder");
	if (orderDineinBtn)
	{
		if(context.cart.items.length == 0){
			showEmptyCartMsg();
		}
	}
    checkCartHasItems();

	if (window.isSTO && !skipSharedCartUpdate && !window.disableSharedCart) {
		if (deleteItem) sharedCartDeleteItem(deleteItem);
		else sharedCartUpdatedDeounce();
	// } else if (window.isSTO && !skipSharedCartUpdate) {
	// 	if (context.cartOpened) showCartSummary();
	}
}

function generateUUID() {
	return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
		const r = Math.random() * 16 | 0;
		const v = c === 'x' ? r : (r & 0x3 | 0x8);
		return v.toString(16);
	}); // didn't use crypto.randomUUID() because it does not support HTTP + local IP: for local test
}

function debounce(fn, delay) {
	var timer = null;
	return function (...args) {
		clearTimeout(timer);
		timer = setTimeout(() => {
		fn.apply(this, args);
		}, delay);
	};
}

//miyu: refactored get linked menu item function created by YZ 
function getOptionsFromLinkedMenuItem(options, menuItems){
	if (options && options.linkedWithDBID)
	{
		var targetOption;
		if (typeof options.linkedWithDBID == 'string')
		{
			targetOption = JSON.parse(getMenuItem(menuItems, options.linkedWithDBID).options);
			if (targetOption)
			{
				options = targetOption.opts;
			}
		} else
		{
			var origOpt = options;
			options = [];
			for (var i = 0; i < origOpt.linkedWithDBID.length; ++i)
			{
				if(!!getMenuItem(menuItems, origOpt.linkedWithDBID[i])){
					var temp = JSON.parse(getMenuItem(menuItems, origOpt.linkedWithDBID[i]).options); 
					if (temp && temp.opts)
						options = options.concat(temp.opts);
				}
			}
		}
	}
	return options;
}

//load option item from menuItem, group by option ctgy and choices
function loadOptionInDetails(menuItem, menuItems, groupedChoiceList, itemDeInfo){
	var optsStrings = JSON.parse(menuItem.options);
	var radioList = [];  //multiple choice 
	var checkboxList = {};   //check or uncheck
	if (optsStrings && optsStrings.opts)
	{
		var options = optsStrings.opts;
		var optCatLimits = optsStrings.optCatNum;
		if (options && options.linkedWithDBID)
		{
			options = getOptionsFromLinkedMenuItem(options, menuItems);
		}
		if (window.showGlobalOptionsInDetails) {
			if (optsStrings.hideGlobalSeasoning) {
				// do nothing
			} else {
				options = options.concat(getGlobalOptions(menuItem));
			}
		}
		// filter option values based on ordering mode
		for (var fi = 0; fi < options.length; fi++) {
			if (options[fi].values) {
				options[fi].values = options[fi].values.filter(function(val) {
					if (window.isOnline && val.hideOnOrderToGo) return false;
					if (window.isSTO && val.hideOnScanToOrder) return false;
					return true;
				});
			}
		}
		// remove option groups with no values left
		options = options.filter(function(opt) { return opt.values && opt.values.length > 0; });

		for(var i = 0; i < options.length ; i++){
			var option = options[i];
			var ctgyname = option.name.toLowerCase();

			//groupedChoiceMax
			if(ctgyname.lastIndexOf("-") != -1){
				addToGroupedChoiceList(groupedChoiceList, ctgyname);
            }
			//if there is only one choice in the option, it is add on (checkbox) else, it's multiple choice (radio)
			if(option.values.length == 1){

				if(!checkboxList.hasOwnProperty(ctgyname)){
					checkboxList[ctgyname] = {
						idx: i,
						stepNum: option.stepNum,
						choices: [],
						preCollapsed: !!(option.preCollapsed),
						numOfRequired: (typeof optCatLimits == 'undefined' || typeof optCatLimits[option.name] == 'undefined' || typeof optCatLimits[option.name].optCatReq == 'undefined') ? 0 : parseInt(optCatLimits[option.name].optCatReq),
						numOfRequiredMax: (typeof optCatLimits == 'undefined' || typeof optCatLimits[option.name] == 'undefined' || typeof optCatLimits[option.name].optCatReqMax == 'undefined') ? 0 : parseInt(optCatLimits[option.name].optCatReqMax)
					}
				} else {
					// if category already exists, propagate preCollapsed if any option requests it
					if (option.preCollapsed)
						checkboxList[ctgyname].preCollapsed = true;
				}
				checkboxList[ctgyname].choices.push(option.values[0]);
				
			} else if (option.values.length > 1){
				var opt = {
					idx: i,
					stepNum: option.stepNum,
					ctgyname: ctgyname,
					choices: option.values,
					isRequired: option.required,
					preCollapsed: !!(option.preCollapsed)
				}
				radioList.push(opt);
			} 
		}
	}
	displayOptionInDetails(checkboxList, radioList, groupedChoiceList, itemDeInfo);
}

// check if the element is visible with in the container.
// return true if it is visible, false if it's not visible
function isScrolledIntoView(elem, container)
{
	var elemTop = $(elem).offset().top;
	var containerOffset = $(container).offset().top;
    return ((elemTop + elem.height() <= containerOffset + 650) && (elemTop >= containerOffset + 5));
}

//common
function getMenuItem(menuItems, item_id){
	for(var i = 0; i < menuItems.length; i++){
		var menuItem;
		if(menuItems[i].id == parseInt(item_id)){
			menuItem = menuItems[i];
			return menuItem;
		}
	}
	return menuItem;
}

// used in the function cartClearAll()
// 02/26/2018 another way to updateUI manually when clear All have been applied
function manuallyUpdateUIForCart_clearAll(obj)
{
	if (obj.length > 0)
	{
		//4/13/2020 Miyu update for all items including alsoshowinCtgy item
		var overlayMenuItems = $('.overlayMenuItem[item-id="' + obj.attr("item-id") + '"]');
		for(var i = 0; i < overlayMenuItems.length ; i++){
			//need to go increate overlay Counter for each item found with attribute based on item-id
			var overlayMenuItem = $(overlayMenuItems[i])[0];
			var innerHTML = $.views.converters.countItem.apply( {linkCtx: {elem: overlayMenuItem}} );
			overlayMenuItem.innerHTML = innerHTML;
			overlayMenuItem.style.display = 'none';
		}


		// var overlayMenuItem = obj.find(".overlayMenuItem")[0];
		// var innerHTML = $.views.converters.countItem.apply( {linkCtx: {elem: overlayMenuItem}} );
		// overlayMenuItem.innerHTML = innerHTML;		
		// overlayMenuItem.style.display = 'none';
	}
	
	var numItemsSpans = $("#numItems").find("span");
	if (numItemsSpans.length > 0)
	{
		numItemsSpans[0].innerHTML = 0; 
	}
}


//common
function getHoursToday(currentRest){
	var date = new Date();
	var day = date.getDay();
	var hours_today_start;
	var hours_today_end;
	switch(day) {
		case 0:
			hours_today_start = currentRest.openHours.sun.start;
			hours_today_end = currentRest.openHours.sun.end;
			break;
		case 1:
			hours_today_start = currentRest.openHours.mon.start;
			hours_today_end = currentRest.openHours.mon.end;
			break;
		case 2:
			hours_today_start = currentRest.openHours.tue.start;
			hours_today_end = currentRest.openHours.tue.end;
			break;
		case 3:
			hours_today_start = currentRest.openHours.wed.start;
			hours_today_end = currentRest.openHours.wed.end;
			break;
		case 4:
			hours_today_start = currentRest.openHours.thu.start;
			hours_today_end = currentRest.openHours.thu.end;
			break;
		case 5:
			hours_today_start = currentRest.openHours.fri.start;
			hours_today_end = currentRest.openHours.fri.end;
			break;
		case 6:
			hours_today_start = currentRest.openHours.sat.start;
			hours_today_end = currentRest.openHours.sat.end;
			break;		
		default:
			hours_today_start = "unknown";
			hours_today_end = "unknown";		
	}

	var hours_string;
	if( hours_today_start != "close" && hours_today_end != "close" ){
		hours_string = hours_today_start + ":00 - " +  hours_today_end + ":00";
	} else {
		hours_string = "close today";
	}
	return hours_string;
}

// confirm before clear all items in the cart
function confirmClearAllItems(){
	$("#clearAllItemsInCart").text((window.isOnline && window.idxLang == '1') ?"再次点击以清除所有项目" : "Click here again to clear all items");
	$("#clearAllItemsInCart").css("text-decoration","underline");
	$("#clearAllItemsInCart").css("color","#de0000");
	$("#clearAllItemsInCart").addClass("confirmedClearAll-btn");

	$(".confirmedClearAll-btn").tap(function(){
		cartClearAllClicked();
		if (window.isSTO && !window.disableSharedCart) clearSharedCart();
	});	
}
function removeOrderToken()
{
	context.cart.orderToken = null;
	context.cart.tip = -1;
}

function getTaxTotal(){
	var cartItem = context.cart.sortedItems;
	var totalTax = 0;
	for(var i = 0; i < cartItem.length; i++){
		totalTax += cartItem[i].price * ((cartItem[i].taxrate != null && cartItem[i].taxrate !== '') ? cartItem[i].taxrate : context.taxRatio);
	}
	return precise_round(totalTax, 2);
}

//there is an item with different taxrate, culcurate taxTotal
function checkTaxrate(){
	context.cart.subtotal = precise_round(context.cart.subtotal, 2);
	context.cart.taxTotal = getTaxTotal();	
	if (context.cart.isStoOrder){
		context.cart.taxTotal = precise_round(context.cart.taxTotal + context.taxRatio * (context.cart.adjustment || 0), 2);
		if (window.enableAutoTipWithTax) {
			context.cart.taxTotal = precise_round(context.cart.taxTotal + context.taxRatio * (context.cart.tip || 0), 2);
		}
	}
	context.cart.total = context.cart.subtotal + context.cart.taxTotal + (context.cart.subtotal > 0 && context.cart.additionalFeesAmount ? context.cart.additionalFeesAmount : 0);
}

function checkMinimumPayAdjustment(){
	if (window.extraFeeAdjFunc)
	{
		var adjustment = window.extraFeeAdjFunc();
		if (adjustment != null) {
			context.cart.adjustment = adjustment.value;
			context.cart.extraFeeTag = adjustment.name;
			context.cart.total = context.cart.total + adjustment.value;
		} else
		{
			context.cart.adjustment = null;
			context.cart.extraFeeTag = null;
		}
	}
}


//only used in all-code & order.base
function disableBodyScroll(){
	// save scroll position
	scrollAllPageSaverForSafari = $(window).scrollTop();
	// perfect for non-ios device;
	$('html, body').css('overflow','hidden');
	// for safari add below code for disabling overflow, but chrome position will change.
	$('html, body').css('position','relative');
}

function enableBodyScroll(){
	$('html, body').css('overflow','auto');
	// for safari
	$('html, body').css('position','static');
	// scroll back to saved position
	setTimeout(() => {
		$(window).scrollTop(scrollAllPageSaverForSafari || scrollposition);
	}, 0);
}

// note both '+' button and the 'add' button in the cart can trigger this
function cartAddButtonClicked(){
	var itemToAdd = getMenuItemDiv(this, $.view(this).index);
	var menuItemDiv = itemToAdd.menuItemDiv;
	var index = itemToAdd.index;
	var item = context.cart.items[index];
	if (item.maxQuantRestriction){
		if (checkRestricted(item.maxQuantRestriction, item.id, 1)){
			displayRestrictedWarning(item.maxQuantRestriction);
			return;
		} else{
			var itemToAdd = {
				id: item.id, 
				item_id: item.item_id, 
				name: item.name, 
				price: item.price, 
				optionsstr: item.optionsstr, 
				optionitemids: item.optionitemids, 
				optionitemobjects: item.optionitemobjects, 
				specialIns: item.specialIns, 
				taxrate: item.taxrate, 
				itemIndex: item.itemIndex, 
				optionPriceList: item.optionPriceList, 
				togo: item.togo,
				maxQuantRestriction: item.maxQuantRestriction,
				onlineFee: item.onlineFee,
			}
			if (window.isSTO && !window.disableSharedCart) itemToAdd.uuid = generateUUID();
			context.cart.items.push(itemToAdd);
		}
	} else{
		var itemToAdd = {
			id: item.id, 
			item_id: item.item_id, 
			name: item.name, 
			price: item.price, 
			optionsstr: item.optionsstr, 
			optionitemids: item.optionitemids, 
			optionitemobjects: item.optionitemobjects, 
			specialIns: item.specialIns, 
			taxrate: item.taxrate, 
			itemIndex: item.itemIndex, 
			optionPriceList: item.optionPriceList, 
			togo: item.togo,
			onlineFee: item.onlineFee,
		}
		if (window.isSTO && !window.disableSharedCart) itemToAdd.uuid = generateUUID();
		context.cart.items.push(itemToAdd);
	}
	removeOrderToken();
	context.cart.subtotal = context.cart.subtotal + item.price;
	if (context.cart.items.length == 0)
		context.cart.subtotal = 0;
	
	saveCart(currentRest, context.cart);
	saveAndUpdateCart(menuItemDiv)	
}


// note both '-' button and the 'remove' button in the cart can trigger this
function cartDeleteButtonClicked(){
	// We cannot use the below way to get index because attr("index") won't get updated after deleting.
	//var index = $(this).attr("index");
	var itemToAdd = getMenuItemDiv(this, $.view(this).index);
	var menuItemDiv = itemToAdd.menuItemDiv;
	var index = itemToAdd.index;
	var item = context.cart.items[index];
	removeOrderToken();

	context.cart.items.splice(index,1);
	context.cart.subtotal = context.cart.subtotal - item.price;
	if (context.cart.items.length == 0)
		context.cart.subtotal = 0;
	saveCart(currentRest, context.cart);
	if (window.isSTO && !window.disableSharedCart) saveAndUpdateCart(menuItemDiv, item);
	else saveAndUpdateCart(menuItemDiv);
	manuallyUpdateUIForCart($(menuItemDiv), null, true);
}

//refactored from artDeleteButtonClicked
function getMenuItemDiv(obj, index){
	if (!$(obj).hasClass("notcartbtn"))
	{
		// this delete action is coming from cart, instead of the '-' button
		// this index is the index of the sortedItems, need to map it back to the index of item in cart
		index = context.cart.sortedItems[index].index;
		menuItemDiv = $(".menuitem[item-id='" + context.cart.items[index].id + "']")
	} else
	{
		// this index is the index of context.items, need to map it back to the index of item in cart
		if (index == null)
			index = $(obj).parent().attr("id");
		var id = context.items[index].id;
		var found = false;
		for (var i = 0; i < context.cart.items.length; ++i){
			if (id == context.cart.items[i].id){
				index = i;
				found = true;
				break;
			}
		}
		if (!found) // this item is not currently in the cart, so nothing to remove
			return;
		menuItemDiv = obj;
		while (!$(menuItemDiv).hasClass("menuitem"))
			menuItemDiv = menuItemDiv.parentElement;
	}
	return {menuItemDiv: menuItemDiv, index: index};
}
//refactored code so that selforder can overwrite this part
function saveAndUpdateCart(menuItemDiv, deleteItem){
	if (deleteItem) manuallyUpdateUIForCart($(menuItemDiv), deleteItem);
	else manuallyUpdateUIForCart($(menuItemDiv));
	if (!$(this).hasClass("notcartbtn"))
	{
		drawSummaryCart();
		onCartSummaryLoaded();
	}
}


function onCartSummaryLoaded(){
	sortAndMergeMenuItems(context.cart);
	//dinein 
	$(".deletebtn_dinein").tap(cartDeleteButtonClicked);
	$(".cart-addbtn_dinein").tap(cartAddButtonClicked);

	$("#clearAllItemsInCart").tap(function(){
		confirmClearAllItems();
	});

	$(".dinein-ordersummary-table tbody tr").tap(function(){
		if(!$(this).children().hasClass("subtotal")){
			$(this).addClass("updateItem");
			$('#modalConfirm').modal("hide");
			$(".container-fluid").css("overflow-y","unset");
			editItemInCart(this);
		}
	});
}

function editItemInCart(obj){
	var index = $(obj).attr("index");
	var itemIndex = $(obj).attr("itemindex");
	var itemInCart = context.cart.sortedItems[index];
	var menuItem = getMenuItem(context.items, itemInCart.id);

	var itemDeInfo = {
		rowindex: index,  //the index in menuitem list
		itemindex: itemIndex,  //the index for sortedCart when you tap
		menuitem: menuItem,
		originalPrice: menuItem.price,
		curPrice: menuItem.price,
		itemcount: itemInCart.qty,
		optionsObj: JSON.parse(menuItem.options),
		itemincart: itemInCart,
		editfromcart: true,
		optionPriceList: itemInCart.optionPriceList //store selected option item with price
	}
	showDescriptionPage(itemDeInfo, context, 0);
}

// 9/16/2019: miyu 
// get item options info from cart and append current status to menu details 
// use exsisting click() / tap() to manually click() them instead of changing UI to avoid breaking current logic
function appendCurrentOptionChoice(itemDeInfo){
	if(window.isOnline) {
	// add optionpricelist if the item comes from order again that does not have optionpricelist
	if (itemDeInfo.optionPriceList == {} && (itemDeInfo.itemincart.optionsstr != "" || itemDeInfo.itemincart.optionitemobjects.length != 0)) {
		var _optionPriceList = {};
		// add non price options
		var _options = itemDeInfo.optionsstr.split(",").map(op => op.replace(/^\s+/g, ''));
		// remove duplicates since optionpricelist does not need count number
		_options = [...new Set(_options)];
		for (var v of _options) {
			var option_name;
			if ($('#uv_optionsTable-container input[value="' + v +'"]').attr('type') == 'radio')
				option_name = $('#uv_optionsTable-container input[value="' + v +'"]').attr('option_name');
			else
				option_name = $('#uv_optionsTable-container input[value="' + v +'"]').attr('option_name') + v;
			var name = $('#uv_optionsTable-container input[value="' + v +'"]').attr('name');
			var price = '0';
			_optionPriceList[option_name] = {'name': name, 'price': price};
		}
		// add price options
		if (itemincart.optionitemobjects.length != 0) {
			for(var v of itemincart.optionitemobjects) {
				var option_name;
				if ($('#uv_optionsTable-container input[value="' + v.name +'"]').attr('type') == 'radio')
					option_name = $('#uv_optionsTable-container input[value="' + v.name +'"]').attr('option_name');
				else
					option_name = $('#uv_optionsTable-container input[value="' + v.name +'"]').attr('option_name') + v.name;

					var name = $('#uv_optionsTable-container input[value="' + v.name +'"]').attr('name');
					var price = parseInt($('#uv_optionsTable-container input[value="' + v.name +'"]').attr('price'));
				if (_optionPriceList.find(o => o.name === option_name)) {
					_optionPriceList[option_name].price += price;
				}
				else {
					_optionPriceList[option_name] = {'name': name, 'price': price};
				}
			}
		}
		itemDeInfo.optionPriceList = _optionPriceList;
	}
	if (itemDeInfo.itemindex == "") {
		var id = parseInt($('#add_and_remove_Contianer').attr('item-id'));
		itemDeInfo.itemindex = context.items.map(item => item.id).indexOf(id);
	}
	}

	var itemInCart = itemDeInfo.itemincart;
	if(isSelfOrderForDonutSpecialUI(itemDeInfo)) {
		appendDonutChoice(itemInCart, itemDeInfo);
		return;
	}

	//append non-priced options
	if(itemInCart.optionsstr != ""){
		var options = itemInCart.optionsstr.split(", ");
		for(var i = 0; i < options.length; i++){
			var optionname = options[i];
			var $input = $('#uv_optionsTable-container input[value="' + optionname +'"]');
			if($input.length === 0) continue;
			var checked = $input.prop("checked");
			if(checked){
				var $btn = $input.closest('tr').next().find('.plus-option');
				changeOptionCountTapped($btn, itemDeInfo, true);
			} else {
				$('#uv_optionsTable-container td:not([price]) input[value="' + optionname +'"]').click();
				if($input.prop("type") == "radio"){
					radioboxChanged($input[0], itemDeInfo);
				} else {
					//checkbox
					checkboxChanged($input[0], itemDeInfo);
				}
			}
		}
	}

	//append priced options
	var optionitemobjects = itemInCart.optionitemobjects;
	for(var i = 0; i < optionitemobjects.length; i++){
		var optionname = optionitemobjects[i].name;
		var db_id = optionitemobjects[i].db_id;
		var optionprice = optionitemobjects[i].price;
		var count = Number(optionitemobjects[i].count);
		// try match by db_id first, fallback to value (name) only
		var $input = $('#uv_optionsTable-container input[value="' + optionname +'"][db_id="'+db_id+'"]');
		if($input.length === 0){
			$input = $('#uv_optionsTable-container input[value="' + optionname +'"]');
		}
		if($input.length === 0) continue;
		var checked = $input.prop("checked");
		if(count > 1 && checked){
			var $btn = $input.closest('tr').next().find('.plus-option');
			changeOptionCountTapped($btn, itemDeInfo, true);
		} else {
			if(!checked) $input.click();
			if($input.prop("type") == "radio"){
				radioboxChanged($input[0], itemDeInfo);
			} else {
				//checkbox
				checkboxChanged($input[0], itemDeInfo);
			}
		}
	}
		
	//append special ins
	var specialIns = itemInCart.specialIns;
	$('.specialIns_text').val(specialIns);

	//append current qty to itemCount
	$("#item_details_page #itemCount").html("<a>" + itemInCart.qty + "</a>");
	$("#item_details_page .item_price_final").html("<a>$" + precise_round(itemInCart.price, 2).toFixed(2) + "</a>");

}
function checkIsSoldout(l_options, restname) {
	try {
		if (typeof l_options == "string")
		{
			l_options = JSON.parse(l_options);
		}		
		if (typeof restname != "undefined") {
			if (l_options && l_options.soldoutForRest && l_options.soldoutForRest.includes(restname)) {
				return true;
			}
		}
		if (context && context.rest && context.rest.name) {
			if (l_options && l_options.soldoutForRest && l_options.soldoutForRest.includes(context.rest.name)) {
				return true;
			}
		}
	} catch (err) { }

	return false;
}
function loadDescription(target, context, item_id, tempScrollTop){
	var menuItems = context.items;	
	var menuItem = getMenuItem(menuItems, item_id);
	var index = target.attributes.index;
	var isSoldOut = checkIsSoldout(menuItem.options, context.rest.name);
	var optsStrings = JSON.parse(menuItem.options)
	var todayDate = new Date();
	var todayMillisecond = new Date((todayDate.getMonth() + 1) + '/' + todayDate.getDate() + '/' + todayDate.getFullYear()).getTime();
	var currentTime = todayDate.getTime();
	var dayOfTheWeek = todayDate.getDay();
	var displayThisDish = true;
	if (optsStrings)
	{
		var availableDays = [];
		var availableTime = [];
		if (optsStrings.dishDate)
		{
			getAvailableDays(optsStrings, availableDays);
		}
		if (optsStrings.dishHour)
		{
			getAvailableTime(optsStrings, availableTime, todayMillisecond);
		}

		displayThisDish = checkAvailable(availableDays, availableTime, currentTime, dayOfTheWeek, optsStrings, todayMillisecond);
	}
	if (isSoldOut) {
		displaySoldOutWarning();
		$(".container-fluid").css("overflow-y", "unset");  
		if(!!tempScrollTop){
			$(window).scrollTop(tempScrollTop);
		}
		return;
	}
	if (displayThisDish)
	{
		// 09/18/2019 miyu: in order to add edit cart func & refactor 
		// createing obj for itemDeInfo to prevent pass by value 
		var itemDeInfo = {
			rowindex: index.value,
			itemindex: index.value,  //the index for sortedCart when you tap
			menuitem: menuItem,
			originalPrice: menuItem.price,
			curPrice: menuItem.price,
			itemcount: 1,
			optionsObj: JSON.parse(menuItem.options),
			itemincart: null,
			editfromcart: false,
			optionPriceList: {} //store selected option item with price
		}
		showDescriptionPage(itemDeInfo, context, tempScrollTop);
	} else
	{
		displayUnavailableWarning(optsStrings.dishDate, optsStrings.dishHour, optsStrings.dishHourDate);
		$(".container-fluid").css("overflow-y", "unset");  
		if(!!tempScrollTop){
			$(window).scrollTop(tempScrollTop);
		}
	}
}

//common to all-code & base. 
function showDescriptionPage(itemDeInfo, context, tempScrollTop){
	disableBodyScroll();
	loadMenuDetailsTemp(itemDeInfo, context, tempScrollTop);
}

function isSelfOrderForDonutSpecialUI(itemDeInfo) {
    if(itemDeInfo.optionsObj && itemDeInfo.optionsObj.specialUI && itemDeInfo.optionsObj.specialUI == "donut" && location.href.includes('selforderkiosk')) {
        return true;
    }
    return false;
}

function localize(str)
{
	if (window.enableDualLang && window._ld && window._ld[str] && window._ld[str][window.idxLang])
	{
		return window._ld[str][window.idxLang];
	} else {
		return str;
	}
}

//only selforder 
//display option item 
function displayOptionInDetails(checkboxList, radioList, groupedChoiceList, itemDeInfo){
	// for donut
	if(isSelfOrderForDonutSpecialUI(itemDeInfo)) {
		displayOptionInDetailsForDonut(checkboxList, itemDeInfo);
		return;
	}
	if (typeof restname == "undefined") {
		window.restname = context.rest.name;
	}
	var container = $("#uv_optionsTable-container");

	// helper to show +/- and count badge on category header
	function _updateCtgyIndicator($table){
		var $td = $table.find('thead .uv_ctgyLabel').first();
		if($td.length === 0) return;
		var $badge = $td.find('.uv_toggle_indicator');
		if($badge.length === 0){
			$badge = $('<span class="uv_toggle_indicator" style="float:right;opacity:.7;font-weight:600;color:#2f2f2f;margin-right:20px;">');
			$td.append($badge);
		}
		if($table.hasClass('collapsed')){
			var count = $table.find('tbody tr').not('.option-counter-row').length;
			$badge.text('+');
		}else{
			$badge.text('−');
		}
	}

	// Enable collapse/expand of options by tapping the header label (uv_ctgyLabel)
	// Use event delegation to support dynamically generated tables and mobile taps
	container.off('click.uvToggle tap.uvToggle', 'thead .uv_ctgyLabel');
	container.on('click.uvToggle tap.uvToggle', 'thead .uv_ctgyLabel', function(e){
		// Avoid triggering when clicking on interactive elements inside the header
		if($(e.target).is('input,button,.minimum-label,.required-label,.groupChar,span a')) return;
		var $table = $(this).closest('table');
		var $tbody = $table.find('tbody');
		if($tbody.is(':visible')){
			$tbody.slideUp(0);
			$table.addClass('collapsed');
			_updateCtgyIndicator($table);
		}else{
			$tbody.slideDown(0);
			$table.removeClass('collapsed');
			_updateCtgyIndicator($table);
		}
	});

	//display grouped option minium msg
	appendGroupedMinMsg(container, groupedChoiceList);
	var tables = [];
	
	//radio options
	for(var i = 0; i < radioList.length; i++){
		var tableHolder = $('<div>');
		tableHolder.attr("idx", radioList[i].idx);
		tableHolder.addClass('uv_optionsTable-holder');
		var table = $('<table>');
		table.attr("option_name",radioList[i].ctgyname);
		table.attr("table_index",i);
		table.attr("table_type","radio");
		if (radioList[i].stepNum)
			table.attr("stepNum", radioList[i].stepNum);

		// for groupedChoiceList 
		if(radioList[i].ctgyname.lastIndexOf("-") > 0){
			appendGroupedChoiceAttr(table, radioList[i].ctgyname, groupedChoiceList);
		}
		//thead 
		var thead = $('<thead>');
		var tr = $('<tr>');
		var td_name = $('<td colspan="2" class="uv_ctgyLabel">');
		td_name.append(radioList[i].ctgyname);
		// var td_req = $('<td>');
			if(radioList[i].isRequired){
				//if it's required option
				var div_required = $('<div class="ordertogoTheme required-label">');
				var req_span = $('<span>');
				table.addClass("requiredOption");
				req_span.append("REQUIRED");
				div_required.append(req_span);
				td_name.append(div_required);
			}	
		tr.append(td_name);
		// tr.append(td_req);
		thead.append(tr);
		table.append(thead);
		//tbody
		var tbody = $('<tbody>');
		for(var j = 0; j < radioList[i].choices.length ; j++){
			var choice = radioList[i].choices[j];
			var tr = $('<tr>');
			var td_name = $('<td>');
			var label_choice = null, input = null;
			if (choice.db_id && context.itemsWithId) {
				var _menu = context.itemsWithId[choice.db_id];
				if (_menu && _menu.options && _menu.options.indexOf("soldoutForRest") >= 0) {
					var _options = JSON.parse(_menu.options);
					if (_options.soldoutForRest && _options.soldoutForRest.includes(restname)) {
						label_choice = $('<label class="checkbox_container soldoutOption">');
						input = $('<input type="radio" disabled></input>');
					}
				}
			}
			if (label_choice == null)
				label_choice = $('<label class="checkbox_container">');
			if (input == null) 
				if (choice.isDefault && !itemDeInfo.editfromcart)
					input = $('<input type="radio" checked>');
				else
					input = $('<input type="radio">');
				input.attr("name","radio" + i);
				input.attr("value", choice.name.replaceAll('"', " "));
				input.attr("printer", choice.printer);
				input.attr("option_name", radioList[i].ctgyname);
				input.attr("onchange", choice.script_code);
				if (window.enableDualLang && window.idxLang != '0') {
					label_choice.append(choice.altname || choice.name);
				} else {
					label_choice.append(choice.name);
				}
				label_choice.append(input);
				var label_span = $('<span class="checkedradio"></span>');
				label_choice.append(label_span);
			td_name.append(label_choice);
				
			//if there is price
			var td_price = $('<td>');
			if(choice.price){
				td_name.attr("price", choice.price);
				var req_span = $('<span class="uv_optionsPrice font-SSP">');
				req_span.append("+$" + precise_round_str(choice.price, 2));
				input.attr("price", choice.price);
				input.attr("db_id", choice.db_id);
				td_price.append(req_span);
			}
			tr.attr("index",j);
			if (choice.isDefault && !itemDeInfo.editfromcart) tr.attr("isDefault", "1");
			tr.append(td_name);
			tr.append(td_price);
			tbody.append(tr);
		}

		table.append(tbody);

		// Pre-collapse if requested
		if (radioList[i].preCollapsed) {
			tbody.hide();
			table.addClass('collapsed');
		}

		tableHolder.append(table);
		//container.append(tableHolder);
		tables.push(tableHolder);
	}
	
	var index = 0;
	//checkbox options
	//wider option view when there is many options to choose
	var wideviewOn = false;
	if(Object.keys(checkboxList).length >= 2 ){
		for(ctgyname in checkboxList){
			if(checkboxList[ctgyname].choices.length >= 6){
				wideviewOn = true;
				break;
			} 
		}
	}

	var count = 0;
	for(ctgyname in checkboxList){
		count++;
		var tableHolder = $('<div>');
		if(wideviewOn){
			if(count%2 == 0){
				tableHolder.addClass('doubletableview-2');
			} else {
				tableHolder.addClass('doubletableview-1');
			}
		}
		tableHolder.attr("idx", checkboxList[ctgyname].idx);
		tableHolder.addClass('uv_optionsTable-holder');
		var table = $('<table>');

		// for groupedChoiceList
		var displayctgyname = "";
		var groupChar = "";
		var lastHyphenSignIndex = ctgyname.lastIndexOf("-");
		var charAtLastHyphenSignIndexPlusOne = ctgyname.charAt(lastHyphenSignIndex + 1);
		var charAtLastHyphenSignIndexPlusTwo = ctgyname.charAt(lastHyphenSignIndex + 2);
		if (lastHyphenSignIndex != -1 && charAtLastHyphenSignIndexPlusOne.match(/[a-zA-Z]/) != null && !isNaN(parseInt(charAtLastHyphenSignIndexPlusTwo))) 
		{
			appendGroupedChoiceAttr(table, ctgyname, groupedChoiceList);
			displayctgyname = ctgyname.substring(0, ctgyname.lastIndexOf("-")) + "*"; 
			groupChar = "group " +  ctgyname.charAt(ctgyname.lastIndexOf("-") + 1).toUpperCase();
		} else {
			displayctgyname = ctgyname;
		}

		table.attr("option_name",ctgyname);
		table.attr("table_index",index);
		table.attr("table_type","checkbox");
		if (checkboxList[ctgyname].stepNum)
			table.attr("stepNum", checkboxList[ctgyname].stepNum);
		//thead 
		var thead = $('<thead>');
		var tr = $('<tr >');
		var td_name = $('<td colspan="2" class="uv_ctgyLabel">');
		td_name.append(localize(displayctgyname));
		if(groupChar != ""){
			var grouplabel = $('<span class="groupChar">').text(groupChar);
			td_name.append(grouplabel);
		}
		//var td_req = $('<td>');
		if(checkboxList[ctgyname].numOfRequired || checkboxList[ctgyname].numOfRequiredMax)
		{
			//if it's required option
			var div_required = $('<div class="ordertogoTheme minimum-label">');
			var req_span = $('<span>');
			if (checkboxList[ctgyname].numOfRequired > 0)
			{
				table.addClass("minItemsRequired");
				table.attr('itemsRequired',  + checkboxList[ctgyname].numOfRequired);
			}
			if (checkboxList[ctgyname].numOfRequiredMax > 0)
			{
				table.addClass("maxItemsRequired");
				table.attr('itemsRequiredMax',  + checkboxList[ctgyname].numOfRequiredMax);
			}
			if (checkboxList[ctgyname].numOfRequired == checkboxList[ctgyname].numOfRequiredMax && checkboxList[ctgyname].numOfRequired > 0)
			{
				req_span.append(checkboxList[ctgyname].numOfRequired + " ITEM(S) REQUIRED");
				div_required.append(req_span);
				td_name.append(div_required);
			} else
			{
				if (checkboxList[ctgyname].numOfRequired)
				{
					req_span.append(checkboxList[ctgyname].numOfRequired + " MINIMUM");
				}
				if (checkboxList[ctgyname].numOfRequiredMax)
				{
					req_span.append((checkboxList[ctgyname].numOfRequired > 0 ? ' ' : '') + checkboxList[ctgyname].numOfRequiredMax + " MAXIMUM");
				}
				div_required.append(req_span);
				td_name.append(div_required);
			}
		}

		tr.append(td_name);
		//tr.append(td_req);
		thead.append(tr);
		table.append(thead);
		
		//tbody
		var tbody = $('<tbody>');
		for(var j = 0; j < checkboxList[ctgyname].choices.length ; j++){
			// if(checkboxList[ctgyname].choices.length > 8){
			// 	$(".details-box").addClass("wideoption");
			// 	td_name.attr("colspan",3);
			// 	var choice = checkboxList[ctgyname].choices[j];
			// 	if(j%2 == 0){
			// 		var tr = $('<tr>');
			// 	} 
			// 	var td_name = $('<td>');
			// 	var label_choice = $('<label class="checkbox_container">');
			// 	var input = $('<input type="checkbox" >');
			// 	input.attr("name","checkbox" + j);
			// 	input.attr("value", choice.name);
			// 	input.attr("printer", choice.printer);
			// 	input.attr("option_name", ctgyname);
			// 	label_choice.append(choice.name);
			// 	label_choice.append(input);
			// 	var label_span = $('<span class="checkmark"></span>');
			// 	label_choice.append(label_span);
			// 	td_name.append(label_choice);
					
			// 	//if there is price
			// 	var td_price = $('<td>');
			// 	if(choice.price){
			// 		var req_span = $('<span class="uv_optionsPrice font-SSP">');
			// 		req_span.append("$" + precise_round_str(choice.price, 2));
			// 		req_span.attr("price", choice.price);
			// 		input.attr("price", choice.price);
			// 		input.attr("db_id", choice.db_id);
			// 		td_price.append(req_span);
			// 	}

			// 	// Process the second row too add and remove amount
			// 	if(j % 2 == 0){
			// 		var tr_amount = $('<tr>');
			// 	} 
			// 	var td_amount = $('<td colspan="2">');		
			// 	td_amount.append($('<i class="remove-option fa fa-minus"></i>'));
			// 	td_amount.append($('<span class="amount_to_be_added uv_optionsNum font-SSP">').text(1));
			// 	td_amount.append($('<i class="plus-option fa fa-plus"></i>'));

			// 	//if there is price
			// 	if(choice.price){
			// 		var td_price_added = $('<td>');
			// 		var price_span = $('<span class="uv_optionsPrice font-SSP">');
			// 		price_span.append("$" + precise_round_str(choice.price, 2));
			// 		td_price_added.append(price_span);
			// 		tr_amount.attr("price", choice.price);
			// 	}
			// 	tr_amount.attr("value", choice.name);
			// 	tr_amount.attr("option_name", ctgyname);
			// 	// Appending the second row with add/remove functionality for the current add on
			// 	tr_amount.attr("add-row-index", j);
			// 	tr_amount.addClass("option-counter-row");
			// 	tr_amount.append(td_amount);

			// 	// Appending the first row with information about the item
			// 	tr.attr("index",j);
			// 	tr.append(td_name);
			// 	tr.append(td_price);
			// 	tbody.append(tr);
			// 	tbody.append(tr_amount.hide());
			// } else {
				$(".details-box").removeClass("wideoption");
				var choice = checkboxList[ctgyname].choices[j];
				var tr = $('<tr>');
				var td_name = $('<td>');
				var label_choice = null, input = null;
				if (choice.db_id && context.itemsWithId) {
					var _menu = context.itemsWithId[choice.db_id];
					if (_menu && _menu.options && _menu.options.indexOf("soldoutForRest") >= 0) {
						var _options = JSON.parse(_menu.options);
						if (_options.soldoutForRest && _options.soldoutForRest.includes(restname)) {
							label_choice = $('<label class="checkbox_container soldoutOption"></label>');
							input = $('<input type="checkbox" disabled></input>');
						}
					}
				}
				if (label_choice == null)
					label_choice = $('<label class="checkbox_container">');
				
				if (input == null && choice.isDefault && checkboxList[ctgyname].choices.length == 1 && choice.demulti) {
					input = $('<input type="checkbox" checked disabled>')
				}

				if (input == null) 
					if (choice.isDefault && !itemDeInfo.editfromcart)
						input = $('<input type="checkbox" checked>');
					else
						input = $('<input type="checkbox">');
				input.attr("name","checkbox" + j);
				input.attr("value", choice.name ? choice.name.replaceAll('"', " ") : "");
				input.attr("altname", choice.altname ? choice.altname.replaceAll('"', " ") : "");
				input.attr("printer", choice.printer);
				input.attr("option_name", ctgyname);
				input.attr("onchange", choice.script_code);

				var label_span = $('<span class="checkmark"></span>');			
				label_choice.append(input);
				label_choice.append(label_span);
				if (window.enableDualLang && window.idxLang != '0') {
					label_choice.append(choice.altname || choice.name);
				} else {
					label_choice.append(choice.name);
				}
				td_name.append(label_choice);
					
				//if there is price
				var td_price = $('<td>');
				if(choice.price){
					td_name.attr("price", choice.price);
					var req_span = $('<span class="uv_optionsPrice font-SSP">');
					req_span.append("+$" + precise_round_str(choice.price, 2));
					req_span.attr("price", choice.price);
					input.attr("price", choice.price);
					input.attr("db_id", choice.db_id);
					td_price.append(req_span);
				}

				// Process the second row too add and remove amount
				var tr_amount = $('<tr>');
				var td_amount = $('<td colspan="2">');		
				td_amount.append($('<i class="remove-option fa fa-minus"></i>'));
				td_amount.append($('<span class="amount_to_be_added uv_optionsNum font-SSP">').text(1));
				td_amount.append($('<i class="plus-option fa fa-plus"></i>'));

				//if there is price
				if(choice.price){
					var td_price_added = $('<td>');
					var price_span = $('<span class="uv_optionsPrice font-SSP">');
					price_span.append("$" + precise_round_str(choice.price, 2));
					td_price_added.append(price_span);
					tr_amount.attr("price", choice.price);
				}
				tr_amount.attr("value", choice.name);
				tr_amount.attr("option_name", ctgyname);
				// Appending the second row with add/remove functionality for the current add on
				tr_amount.attr("add-row-index", j);
				if (choice.demulti) tr_amount.attr("demultiselect", true);
				tr_amount.addClass("option-counter-row");
				tr_amount.append(td_amount);

				// Appending the first row with information about the item
				tr.attr("index",j);
				if (choice.isDefault && !itemDeInfo.editfromcart) tr.attr("isDefault", "1");
				tr.append(td_name);
				tr.append(td_price);
				tbody.append(tr);
				tbody.append(tr_amount.hide());
			//}
		}

		table.append(tbody);

		// Pre-collapse if requested for this checkbox category
		if (checkboxList[ctgyname].preCollapsed) {
			tbody.hide();
			table.addClass('collapsed');
		}

		tableHolder.append(table);
		//container.append(tableHolder);
		tables.push(tableHolder);
		index++;
	}
	if(wideviewOn){
		$(".details-box").addClass("wideoption");
	}
	
	tables.sort(function(a, b){
		var x = parseInt(a.attr("idx"));
		var y = parseInt(b.attr("idx"));
		return x - y;
	});
	for (var i = 0; i < tables.length; ++i){
		container.append(tables[i]);
		_updateCtgyIndicator($(tables[i]).find('table'));
	}
}

//common codes
function appendMenuItemToDetailsPage(itemDeInfo, context){
	if (itemDeInfo.optionsObj.pricePerUnit) {
		$("#weight_input_div").css("display", "");
		$(".item_price").css("display", "none");
		$("#add_and_remove_Contianer").css("display", "none");
		$("#addToGlobalCart").addClass('disable-btn');
		$("#addToGlobalCart").attr('pricePerUnit', itemDeInfo.optionsObj.pricePerUnit);
		$("#pricePerUnit_div").css("display", "")
		$("#pricePerUnit_div label").text("$" + itemDeInfo.optionsObj.pricePerUnit + " per lb");
	}
	var newItemCount = itemDeInfo.itemcount;
	var menuItem =  itemDeInfo.menuitem;
	$("#item_details_page #itemCount").html("<a>" + newItemCount + "</a>");
	updateItemsTotalPriceInDetails(newItemCount, itemDeInfo.curPrice);
	
	var item_name = menuItem.name;
	$("#item_details_page .dishNameHolder").html("<a>" + item_name + "</a>");
	$("#item_details_page .dishNameHolder a").addClass("item_details_text");
	//$("#item_details_page .dishNameHolder a").css("color","#2F2F2F");
	$("#addToGlobalCart").attr("index", itemDeInfo.rowindex);
	$("#addToGlobalCart").attr("itemindex", itemDeInfo.itemindex);
	
	var skipAddiImgForDonutOnline = false;
	if(itemDeInfo.optionsObj.specialUI && typeof isOnline != 'undefined' && isOnline) {
		skipAddiImgForDonutOnline = true;
	}
	//mengjiao
	var images = [];
	var additionalImg = itemDeInfo.optionsObj.additionalImg;
	if (additionalImg && additionalImg.length > 0 && !skipAddiImgForDonutOnline) {
		images = additionalImg;
	} else {
		images.push(itemDeInfo.optionsObj.image);
	}

	applyImageToDetailPage(images);

	var description = itemDeInfo.optionsObj.descriptions;
	if(description){
		$('#item_details_page .dishDetailsHolder').html("<a>" + description + "</a>");
		//$('#item_details_page .dishDetailsHolder a').css("color","#585858");
	}

	var groupedChoiceList = {}; //stores groupedChoice for max num of item have to be chosen
	//05/01/2018: MIYU: option added in user visible
	loadOptionInDetails(menuItem, context.allitems, groupedChoiceList, itemDeInfo);

	if(itemDeInfo.editfromcart && itemDeInfo.itemincart != null){
		appendCurrentOptionChoice(itemDeInfo);
		$('.btn-footer-addToCart div:nth-child(2)').text("Update Item");
		$('.btn-footer-addToCart div:nth-child(1) a').text("更新");
		$("#addToGlobalCart").attr("index", itemDeInfo.rowindex);
		$("#addToGlobalCart").attr("itemindex", itemDeInfo.itemindex);
		$("#addToGlobalCart").addClass("updateItem");
		$("#addToGlobalCart").addClass("updateItem");
		$('#backToMenu').addClass("updateItem");
	}
	disableHeaderMove = false;
	checkiOSsafari();
	onMenuDetailsLoaded(itemDeInfo, groupedChoiceList);
}

function applyImageToDetailPage(img){
	if (!img[0]) {
		$("#item_details_page #item_image").css("display","none");
		$("#item_details_page #backToMenu").removeClass("header_darkfilter");
		// $("#item_details_page #backToMenu").addClass("header_nofilter");		
	} else {
		if (img.length == 1) {
			$("#item_details_page #item_image").css("display", "block");
			$("#item_details_page #item_image").attr("src",img[0]);
			$("#item_details_page .item-slide-wrapper").removeClass("show");
		} else {
			$("#item_details_page #item_image").css("display", "none");
			$("#item_details_page .item-slide-wrapper").addClass("show");
			for (var i = 0; i < img.length; i++) {
				$("#item_details_page .item-slide-wrapper").append("<div><img class='slide-image' src='"+ img[i] + "') /></div>");
			}
			$('.item-slide-wrapper').slick({
				slidesToShow: 1,
				slidesToScroll: 1,
				swipe: true,
				dots: true,
				arrows: false,
				speed: 300,
				indefinite: false,
				adaptiveHeight: true
			})
		}
	}
}

function loadMenuDetailsTemp(itemDeInfo, context, tempScrollTop){
	hideElemsWhenMenuDetailShows();
	$("#search_input").prop('disabled', true);
	setTimeout(() => {
		$("#specialIns_input").prop('disabled', false);
	}, 200);

	$("#item_details_page_container").removeClass("hide-banner");	
	$("#item_details_page").removeClass("hide-banner");	
	$("#item_details_page_filter").removeClass("hide-banner");
	$("#item_details_page").css("transition",  "all 0.2s");		
	$("#item_details_page").attr("scrollposition", tempScrollTop);
	{
		var opt = null;
		if (itemDeInfo.menuitem.options != null)
			opt = JSON.parse(itemDeInfo.menuitem.options);
		if (opt.dishDetailURL){
			itemDeInfo.menuitem.dishDetailURL = opt.dishDetailURL;
			fetch(opt.dishDetailURL).then(function (response) {
				response.text().then(function (text) {
					$("#dishDetailURLContext").html(text);
				});
			});
		}
	}
	var renderMenu = $.templates.menuDetails.render(itemDeInfo.menuitem);
	$("#item_details_page").html(renderMenu).promise().done(function(){
		setTimeout(() => {
			appendMenuItemToDetailsPage(itemDeInfo, context);
		}, 0);
	});
}

// save scroll position and adjust header banner depends, then load Description page
// need to return disableHeaderMove to update boolean
function saveScrollandloadDetails(target, disableHeaderMove){
	var item_id = $(target).attr("item-id");
	var tempScrollTop = $(window).scrollTop();  //save scroll position
	$(".container-fluid").css("overflow-y","hidden");  //scroll to 0
	$(".container-fluid").scrollTop(tempScrollTop);

	if(tempScrollTop >= 200){
		disableHeaderMove = true;
		$(".header-banner").removeClass('header-down').removeClass('header-up');
		$(".header-banner").addClass('header-up-nomove');
	}
	loadDescription(target, context, item_id, tempScrollTop);	
	return disableHeaderMove;
}

function closeCartSummary(){
	var container = $('#modalConfirm');
	container.modal("hide");
	$(".container-fluid").css("overflow-y","unset");
	$(window).scrollTop(scrollposition);
	enableBodyScroll();
	context.cartOpened = false;
}


//addItemToCart by getting the item from menu by menu id on the + - btn 
function addItemToCart(addbtn, maxQuantRestriction) {
	var l_id = $(addbtn).attr("id");
	_addItemToCartById(l_id, maxQuantRestriction, function () {
		var menuitem = addbtn;
		while (menuitem && !$(menuitem).hasClass("menuitem"))
			menuitem = menuitem.parentElement;
		manuallyUpdateUIForCart($(menuitem));
	});
	return false;
}

function _addItemToCartById(l_id, maxQuantRestriction, callback) {
	var item = context.items[l_id];
	if (item.id != -1) {
		// not giftcard
		function isGiftcardItem() {
			if (context.cart && Array.isArray(context.cart.items))
			for (var item of context.cart.items) {
				if (item.id == -1) return true;
			}
			return false;
		}
		if (isGiftcardItem()) {
			showCustomAlert({
				alerttype: "Alert",
				title: "",
				msg1: 'Your cart contains a gift card. Please note that gift cards cannot be processed in transactions with other items.',
				msg2: ""
			});
			return;
		}
	}
	var itemToAdd = {
		id: item.id,
		item_id: item.item_id,
		name: item.name,
		price: item.price,
		optionsstr: "",
		optionitemids: [],
		optionitemobjects: [],
		specialIns: "",
		taxrate: item.taxrate,
		itemIndex: l_id,
		optionPriceList: {},
		togo: '0',
		onlineFee: item.onlineFee,
	}
	if (window.isSTO && !window.disableSharedCart) itemToAdd.uuid = generateUUID();
	context.cart.items.push(itemToAdd);  //miyu: we need to add all this in order to edit  cart to work
	if (maxQuantRestriction) {
		context.cart.items[context.cart.items.length - 1].maxQuantRestriction = maxQuantRestriction;
	}
	context.cart.subtotal = context.cart.subtotal + item.price;
	
	if (callback)
		callback();

	onMenuLoaded();
	if (typeof cartDeleteButtonClicked == 'function')
		$(".deletebtn").tap(cartDeleteButtonClicked);

	removeOrderToken();

	saveCart(currentRest, context.cart);
}


function radioInputChanged(obj, optionPriceList){
	var selected_option_name = obj.getAttribute("option_name").replaceAll('"'," ");
	if(!optionPriceList.hasOwnProperty(selected_option_name)){
		if(obj.hasAttribute("price")){
			var option = {
				name: obj.getAttribute("name"),
				price: obj.getAttribute("price"),
				value: obj.getAttribute("value"),
			} 
		} else {
			var option = {
				name: obj.getAttribute("name"),
				price: "0",
				value: obj.getAttribute("value"),
			}
		}			
		optionPriceList[selected_option_name] = option;
	} else {
		optionPriceList[selected_option_name].name = obj.getAttribute("name");
		if(obj.hasAttribute("price")){
			optionPriceList[selected_option_name].price = obj.getAttribute("price");
		} else {
			optionPriceList[selected_option_name].price = "0";
		}	
	}
}

//selfroder only for now, include counter, need to appy to all other
function checkboxInputChanged(obj, optionPriceList){
	//check if it has optionPrice
	var selected_option_name = obj.getAttribute("option_name").replaceAll('"'," ") + obj.getAttribute("value");
	var addRow = $(obj).closest("tr").next();
	if (obj.checked) {

		//add
		if(!optionPriceList.hasOwnProperty(selected_option_name)){
			if(obj.hasAttribute("price")){
				var option = {
					name: obj.getAttribute("name"),
					price: obj.getAttribute("price"),
					value: obj.getAttribute("value"),
				} 
			} else {
				var option = {
					name: obj.getAttribute("name"),
					price: "0",
					value: obj.getAttribute("value"),
				}
			}			
			optionPriceList[selected_option_name] = option;
		} else {
			optionPriceList[selected_option_name].name = obj.getAttribute("name");
			if(obj.hasAttribute("price")){
				optionPriceList[selected_option_name].price = obj.getAttribute("price");
			} else {
				optionPriceList[selected_option_name].price = "0";
			}	
		}
		if (addRow.attr("demultiselect") != "true")
			addRow.show();
		addRow.find(".amount_to_be_added").text("1");
		$(obj).attr("count", 1);
	} else {
		// remove the checking
		const $counterRow = $(obj).closest("tr").next(".option-counter-row");
		const $priceTag = $(obj).closest("tr").find(".uv_optionsPrice");
		if($priceTag.length > 0){
			const priceToAdd = Number($(obj).attr("price"));
			if(priceToAdd > 0){
				$priceTag.text('+$' + precise_round_str(priceToAdd, 2));
			}
		}	
		const newOptionCount = 0;
		$counterRow.find(".amount_to_be_added").text(newOptionCount);

		delete optionPriceList[selected_option_name];
		addRow.hide();

		$(obj).closest("tr").find("input").removeAttr("count");
	}
}

// refactored code: when .remove-option / .plus-option clicked
function optionCounterClicked(isAdd, target, optionPriceList){
	$selectedCounterTr = $(target).closest("tr");
	const $optionNameRow = $selectedCounterTr.prev();
	const $priceTag = $optionNameRow.find(".uv_optionsPrice");
	const $counter = $selectedCounterTr.find(".amount_to_be_added.uv_optionsNum");
	const currentCount = Number($counter[0].innerText);
	var newOptionCount;
	if(isAdd){
		newOptionCount = currentCount + 1;
	} else {
		newOptionCount = currentCount - 1;
	}

	// Hide the row and uncheck the current box if this amount is under 0
	if (newOptionCount <= 0) {
		$selectedCounterTr.hide();
		$optionNameRow.find("input").prop("checked", false);	
	} 

	$counter.text(newOptionCount);
	$optionNameRow.find("input").attr("count", newOptionCount);
	if ($priceTag.length > 0) {
		const priceToAdd = Number($selectedCounterTr.attr("price"));
		if(priceToAdd > 0){
			if(newOptionCount > 0){
				$priceTag.text('+$' + precise_round_str(priceToAdd * newOptionCount, 2));
			}
			var selected_option_name = $selectedCounterTr.attr("option_name").replaceAll('"'," ") + $selectedCounterTr.attr("value");
			optionPriceList[selected_option_name].price = Number(priceToAdd * newOptionCount);
			return true;
		}
		return false;	
	}
	return false;
}

var ua = window.navigator.userAgent;
var iOS = !!ua.match(/iPad/i) || !!ua.match(/iPhone/i);
var webkit = !!ua.match(/WebKit/i);
var iOSSafari = iOS && webkit && !ua.match(/CriOS/i);


function checkiOSsafari(){
	if(iOSSafari){
		//fk off safari scroll thought bug
		$('#item_details_page_container').css('position','static');
		setTimeout(() => {
			$('#item_details_page_container').css('position','fixed');
		}, 10);
	}
}

function onMenuDetailsLoaded(itemDeInfo, groupedChoiceList){
	$(".ordertogoTheme").css("background-color", window.stoThemeColor ? window.stoThemeColor : context.rest.orderTogoThemeColor);
	checkiOSsafari();

	$("#backToMenu").unbind().tap(function() {
		if(typeof closeKeyboard != 'undefined' && typeof closeKeyboard == 'function') closeKeyboard();
		closeMenuDetails();
		if($(this).hasClass("updateItem")){
			 //show cart again if it's closed from edit 
			viewCartClicked();
			$('#backToMenu').removeClass("updateItem");
		}
	});
	
	//03/09/2018  Miyu: add plus minus btn in details page, but separates from global itemCount
	
	$("#increaseItemCount").unbind().tap(function() {
		changeItemQtyTapped(itemDeInfo, true);
	});

	$("#decreaseItemCount").unbind().tap(function() {
		changeItemQtyTapped(itemDeInfo, false);
	});

	//updatePrice when option withPrice is selected...
	$('input[type="radio"]').unbind().on('change', function(e) {
		if($(this).attr("name") == "dineinORtogo"){
			// placeholder
		} else {
			radioboxChanged(this, itemDeInfo);
			if (window.radioboxClickedCallback) window.radioboxClickedCallback(this, itemDeInfo);
		}
	});

	if(location.pathname.substr(-6) == "dinein"){
		$("#dineintogo_options_container").css("display", "");
		if (itemDeInfo && itemDeInfo.itemincart && itemDeInfo.itemincart.togo == '1') {
			$("input[name=dineinORtogo][value=togo]").attr("checked", "checked");
		} else {
			$("input[name=dineinORtogo][value=dinein]").attr("checked", "checked");
		}
	}

	$('input[type="checkbox"]').unbind().on('change', function(e) {
		checkboxChanged(this, itemDeInfo);
		checkItemsRequiredAfterEveryClick(this);
		if (window.checkboxClickedCallback) window.checkboxClickedCallback(this, itemDeInfo);
	});

	//option counter -
	$(".remove-option").unbind().tap(function() {
		changeOptionCountTapped(this, itemDeInfo, false);
		checkItemsRequiredAfterEveryClick(this);
	});

	//option counter +
	$(".plus-option").unbind().tap(function() {
		changeOptionCountTapped(this, itemDeInfo, true);
		checkItemsRequiredAfterEveryClick(this);
	});

	function checkItemsRequiredAfterEveryClick(self) {
		//the scroll container is different for mobile & selforder
		var container = getOptionTableHolder();
		//check grouped option first
		var meetGroupedMinium = checkGroupedChoice(groupedChoiceList, container);
		console.log(meetGroupedMinium.groupedItemsRequiredCheck);
		$("#uv_optionsTable-container table:not([style*='display: none']) .plus-option").show();
		$("#uv_optionsTable-container table:not([style*='display: none']) input.itemsRequiredCheck_hide").removeAttr("disabled").removeClass("itemsRequiredCheck_hide").siblings().show();
		for (var category in meetGroupedMinium.groupedItemsRequiredCheck) {
			var groupedItemsRequiredCheck = meetGroupedMinium.groupedItemsRequiredCheck[category];
			if (groupedItemsRequiredCheck.selectedCount >= groupedItemsRequiredCheck.max) {
				// disabling/gery out category select
				// category;
				var need_disabled_category = $("#uv_optionsTable-container table[ctgygroup='" + category + "']");
				need_disabled_category.find('.plus-option').hide();

				need_disabled_category.find('input:not([count]):not([disabled])').siblings().hide();
				need_disabled_category.find('input:not([count]):not([disabled])').attr("disabled", "disabled").addClass("itemsRequiredCheck_hide");

				need_disabled_category.find('input[count="0"]:not([disabled])').siblings().hide();
				need_disabled_category.find('input[count="0"]:not([disabled])').attr("disabled", "disabled").addClass("itemsRequiredCheck_hide");
			}
		}

		//05/01/2018 miyu: instead of changing globalCount, only allow it to add new item
		var selectedOptions = getOptionsstrAndIDs();
		console.log(selectedOptions.itemsRequiredCheck);
		//$("#uv_optionsTable-container .plus-option").show();
		//$("#uv_optionsTable-container input.itemsRequiredCheck_hide").removeAttr("disabled").removeClass("itemsRequiredCheck_hide").siblings().show();
		for (var category in selectedOptions.itemsRequiredCheck) {
			var itemsRequiredCheck = selectedOptions.itemsRequiredCheck[category];
			if (itemsRequiredCheck.selectedCount >= itemsRequiredCheck.max) {
				// disabling/gery out category select
				// category;
				var need_disabled_category = $("#uv_optionsTable-container table[option_name='" + category + "']");
				need_disabled_category.find('.plus-option').hide();

				need_disabled_category.find('input:not([count]):not([disabled])').siblings().hide();
				need_disabled_category.find('input:not([count]):not([disabled])').attr("disabled", "disabled").addClass("itemsRequiredCheck_hide");

				need_disabled_category.find('input[count="0"]:not([disabled])').siblings().hide();
				need_disabled_category.find('input[count="0"]:not([disabled])').attr("disabled", "disabled").addClass("itemsRequiredCheck_hide");
			}
		}

	}

	$(".uv_optionsTable-holder table tr[isdefault=1] input").trigger("change");

	bindSpecialInsEvent(itemDeInfo, groupedChoiceList);
	bindAddToGlobalCart(itemDeInfo, groupedChoiceList);	
	checkItemsRequiredAfterEveryClick();
	if (itemDeInfo.menuitem.id == -1) {
		$(".uv_ctgyLabel").last().text("Recipient's Phone Number")
		$("#specialIns_input").attr("placeholder","For others: leave phone number here");
		$("#specialIns_input").mask('(000) 000-0000');
		// $("#item_image").attr("src",context.rest.config.giftcardSaleImg);
		// $("#item_image").css("display","");
		setTimeout(() => {
			$(".uv_specialIns-holder").css("display","");
		}, 200);
	}
}

function getOptionsstrAndIDs(){
	var result = {
		optionsstr: "",
		optionitemids: [],
		optionitemobjects: [],
		optiontotalprice: 0.00,
		specialIns: "",
		requiredChecked: 0,
		requiredOptions: 0,
		unselectedTableName: [],       //check if required option has been selected, true if non required
		amountAdded: {},
		amountRequired: {},
		optionPriceList: {},
		togo: '0',
		itemsRequiredCheck: {},
	};
	//get result from options 
	$(".uv_optionsTable-holder table").each(function(index){
		var option_name = $(this).attr('option_name');
		var table_type = $(this).attr('table_type');
		var table_index = $(this).attr('table_index');
		var stepnum = $(this).attr('stepnum');
		if (stepnum != undefined && $("#addToGlobalCart").attr("cur_stepnum") != undefined && $("#addToGlobalCart").attr("cur_stepnum") != stepnum) {
			return true;
		}
		var selected = $('.uv_optionsTable-holder table[option_name="' + option_name + '"][table_index="' + table_index + '"][table_type="' + table_type + '"] input:checked');

		var selectedCount = 0;
		if(table_type == "checkbox"){
			selectedCount = getSelectedOptionCount(selected);
		} else {
			selectedCount = selected.length;   //if it is "radio"  nth to do with counter
		}
		if($(this).hasClass("requiredOption") || $(this).hasClass("minItemsRequired") || $(this).hasClass("maxItemsRequired")){
			var hasReq = true; 	
			result.requiredOptions++;
		} else {
			var hasReq = false;
		}

		result.amountRequired[option_name] = {
			amountMax: Number($(this).attr("itemsRequiredMax")),
			amountMin: Number($(this).attr("itemsRequired"))
		}
		result.amountAdded[option_name] = 0;
		
		if(selectedCount > 0){
			//selected 
			if(hasReq){
				//is required 
				if ($(this).hasClass("minItemsRequired") || $(this).hasClass("maxItemsRequired"))
				{
					result.itemsRequiredCheck[option_name] = {
						selectedCount: selectedCount,
						min: $(this).attr("itemsRequired"),
						max: $(this).attr("itemsRequiredMax"),
					}
					if (!$(this).hasClass("maxItemsRequired"))
					{
						if (selectedCount >= $(this).attr("itemsRequired"))
						{
							result.requiredChecked++;
						} else
						{
							if (hasReq && result.requiredOptions > result.requiredChecked)
							{
								result.unselectedTableName.push(option_name);
							}
						}
					}
					if (!$(this).hasClass("minItemsRequired"))
					{
						if (selectedCount <= $(this).attr("itemsRequiredMax"))
						{
							result.requiredChecked++;
						} else
						{
							if (hasReq && result.requiredOptions > result.requiredChecked)
							{
								result.unselectedTableName.push(option_name);
							}
						}
					}
					if ($(this).hasClass("minItemsRequired") && $(this).hasClass("maxItemsRequired"))
					{
						
						if (selectedCount <= $(this).attr("itemsRequiredMax") && selectedCount >= $(this).attr("itemsRequired"))
						{
							result.requiredChecked++;
						} else
						{
							if (hasReq && result.requiredOptions > result.requiredChecked)
							{
								result.unselectedTableName.push(option_name);
							}
						}
					}
				} else
				{
					result.requiredChecked++;
				}			
			} 
			//check if it has price, if so, save the db_id and to optionitemids
			var sumOptionPrice = 0;
			
			for(var i = 0; i < selected.length; i++){
				
				var thisoptioncount = $(selected[i]).attr("count");
				if(!!thisoptioncount){
					thisoptioncount = Number($(selected[i]).attr("count"));
				} else {
					thisoptioncount = selected.length;
				}
				 
				for(var j = 0; j < thisoptioncount ; j++){
					// repeat the n times when there is n count of same options

					if(selected[i].hasAttribute("price")){
						var option = {
							altname: selected[i].getAttribute("altname"),
							name: selected[i].getAttribute("value"),
							price: selected[i].getAttribute("price"),
							db_id: selected[i].getAttribute("db_id"),
							printer: selected[i].getAttribute("printer"),
							count: selected[i].getAttribute("count")
						}
						
						sumOptionPrice += parseFloat(selected[i].getAttribute("price"));
						result.optionitemobjects.push(option);
						result.optionitemids.push(selected[i].getAttribute("db_id"));
					} else {
						if(result.optionsstr == ""){
							result.optionsstr = selected[i].value;
						} else {
							result.optionsstr += ", " + selected[i].value;
						}
					}
				}
			}
			result.optiontotalprice += sumOptionPrice;
		} else {
			if ($(this).hasClass("maxItemsRequired") && !$(this).hasClass("minItemsRequired"))
			{
				result.requiredChecked++;
			}
			if(hasReq && result.requiredOptions > result.requiredChecked){
				result.unselectedTableName.push(option_name);
			}
		}
		
		if (stepnum != undefined && $("#addToGlobalCart").attr("cur_stepnum") != undefined && $("#addToGlobalCart").attr("cur_stepnum") == stepnum) {
			// return false;
		}
	});
	result.specialIns = $("#specialIns_input").val();
	if ($("input[name=dineinORtogo]:checked").val() == "togo") {
		result.togo = "1"
	}
	return result;
}

function genStepnumOnTable(stepnum_arr) {
	if (stepnum_arr.length) {
		var max_step = stepnum_arr[stepnum_arr.length - 1];
		$(".uv_optionsTable-holder table").each(function (i, e) {
			if ($(this).attr('stepnum') == null && !$(this).parent().hasClass("uv_specialIns-holder")) {
				max_step++;
				$(this).attr('stepnum', max_step);
				stepnum_arr.push(max_step);
			}
		});
	}
}

function get_stepnum_array() {
	var stepnum_arr = [];
	$(".uv_optionsTable-holder table[stepnum]").each(function (i, e) {
		stepnum_arr.push($(e).attr("stepnum"));
	});
	stepnum_arr.sort();
	genStepnumOnTable(stepnum_arr);
	return stepnum_arr;
}

function showNextStep(cur_stepnum) {
	if (cur_stepnum == null) cur_stepnum = $("#addToGlobalCart").attr("cur_stepnum") || 0;
	var stepnum_arr = get_stepnum_array();
	if (stepnum_arr.length > 1) {
		var next_stepnum;
		for (var _ of stepnum_arr) {
			if (_ > cur_stepnum) {
				next_stepnum = _;
				break;
			}
		}

		$(".uv_optionsTable-holder table").each(function () {
			if (next_stepnum == null) {
				$(this).show();
			} else {
				if (next_stepnum == $(this).attr('stepnum')) {
					$(this).show();
				} else {
					$(this).hide();
				}
			}
		});
		if (next_stepnum) {
			$("#addToGlobalCart").attr("cur_stepnum", next_stepnum);
			$("#addToGlobalCart div").eq(1).text("Next Step");
			if ($("#addToGlobalCart div a").hasClass("button-2nd"))
				$("#addToGlobalCart div a").eq(0).text("下一步");
		} else {
			$("#addToGlobalCart").removeAttr("cur_stepnum");
			if ($("#addToGlobalCart").hasClass("updateItem")) {
				$("#addToGlobalCart div").eq(1).text("Update Item");
			} else {
				$("#addToGlobalCart").attr("nextstepdone", "1");
				$("#addToGlobalCart div").eq(1).text("Add To Cart");
				if ($("#addToGlobalCart div a").hasClass("button-2nd"))
					$("#addToGlobalCart div a").eq(0).text("加入购物车");
			}
		}
	}
}

//selfroder only
function bindAddToGlobalCart(itemDeInfo, groupedChoiceList){
	function checkSelfOrderOneRquiredItem() {
		if(!$(this).hasClass("updateItem") && location.href.includes('selforderkiosk')) {
			var _left = getRequiredMenuitems("selforder");
			if(_left.length == 1 && _left[0] == itemDeInfo.menuitem.id) {
				return true;
			}
		}
		return false;
	}
	
	function checkOnlineOneRquiredItem() {
		if(!$(this).hasClass("updateItem") && window.isOnline) {
			var _left = getRequiredMenuitems("online");
			if(_left.length == 1 && _left[0] == itemDeInfo.menuitem.id) {
				return true;
			}
		}
		return false;
	}

	function checkStoOneRquiredItem() {
		if(!$(this).hasClass("updateItem") && window.isSTO) {
			var _left = getRequiredMenuitems("sto");
			if(_left.length == 1 && _left[0] == itemDeInfo.menuitem.id) {
				return true;
			}
		}
		return false;
	}
	
	if (checkSelfOrderOneRquiredItem()) {
		$('.addCartText').text('Check Out');
	} else if (checkOnlineOneRquiredItem()) {
		if (window.isUser) {
			$('.addCartText').text('Check Out');
		} else {
			$('.addCartText').text("Sign up / Log in to Checkout");
		}
	} else if (checkStoOneRquiredItem()) {
		if (window.enableSTOLoginRequired) {
			if (window.isUser) {
				$('.addCartText').text('Place Order');
			} else {
				$('.addCartText').text("Sign up / Log in to Place Order");
			}
		} else {
			$('.addCartText').text('Place Order');
		}
	}

	if (!$("#addToGlobalCart").hasClass("updateItem") &&
		$("#addToGlobalCart").attr("cur_stepnum") == undefined &&
		$("#addToGlobalCart").attr("nextstepdone") != '1'
	) {
		showNextStep();
	}
	// update cart with global item count!! 
	$("#addToGlobalCart").unbind().tap(function() {
		if(typeof closeKeyboard != 'undefined' && typeof closeKeyboard == 'function') closeKeyboard();
		var itemindex = $(this).attr("itemindex");
		if (itemDeInfo.menuitem.id == -1) {
			function hasNoneGiftcardItem() {
				if (context.cart && Array.isArray(context.cart.items))
				for (var item of context.cart.items) {
					if (item.id != -1) return true;
				}
				return false;
			}

			if (hasNoneGiftcardItem()) {
				showCustomAlert({
					alerttype: "Alert",
					title: "",
					msg1: 'Please check your cart. Gift cards cannot be processed in transactions with other items.',
					msg2: ""
				});
				return;
			}

			if ($("input[value='For others']").prop("checked") && $("#specialIns_input").val().replace(/[^0-9]/gi, '').length != 10) {
				// showCustomAlert({
				// 	alerttype: "Alert",
				// 	title: "",
				// 	msg1: 'Since you have chosen the "For others" option, please enter the RECIPIENT\'S PHONE NUMBER.',
				// 	msg2: ""
				// });
				shakeDomObject($("#specialIns_input"));
				return;
			}
			if ($("input[value='For myself']").prop("checked")) {
				$("#specialIns_input").val('')
			}
		} else {
			// not giftcard
			function isGiftcardItem() {
				if (context.cart && Array.isArray(context.cart.items))
				for (var item of context.cart.items) {
					if (item.id == -1) return true;
				}
				return false;
			}
			if (isGiftcardItem()) {
				showCustomAlert({
					alerttype: "Alert",
					title: "",
					msg1: 'Your cart contains a gift card. Please note that gift cards cannot be processed in transactions with other items.',
					msg2: ""
				});
				return;
			}
		}
		if(isSelfOrderForDonutSpecialUI(itemDeInfo)) {
			donutAddToCartClicked(this);
			return;
		}
		if(!$("#addToGlobalCart").hasClass('disable-btn')){
			
			//the scroll container is different for mobile & selforder
			var container = getOptionTableHolder(); 
			
			//check grouped option first
			var meetGroupedMinium = checkGroupedChoice(groupedChoiceList, container);
			if(meetGroupedMinium.withinMax){
				//05/01/2018 miyu: instead of changing globalCount, only allow it to add new item
				var selectedOptions = getOptionsstrAndIDs();
				
				// save optionPriceList to seletectedOptions and push to cart items for edit cart
				selectedOptions.optionPriceList = itemDeInfo.optionPriceList;
				var numNewItem = parseInt($('#itemCount a').text());
				var itemindex = $(this).attr("itemindex");

				var menuItem = context.items[itemindex];
				var hasMaxRestriction = false;
				var maxQuantRestriction = 0;
				if (menuItem.options)
				{
					var optionJson = JSON.parse(menuItem.options);
					if (optionJson.maxQuantRestriction)
					{
						maxQuantRestriction = optionJson.maxQuantRestriction;
						var numOfItemsRestricted = false;	
						if($(this).hasClass("updateItem")){		
							numOfItemsRestricted = checkRestricted(maxQuantRestriction, menuItem.id, numNewItem, true);
						} else {
							numOfItemsRestricted = checkRestricted(maxQuantRestriction, menuItem.id, numNewItem);
						}
						if(numOfItemsRestricted){
							displayRestrictedWarning(maxQuantRestriction);
							return;
						}
						hasMaxRestriction = true;
					}
				}

				var weight = null;
				if ($("#weight_input").val()) {
					weight = parseFloat($("#weight_input").val());
				}

				//check if the required option has been selected, if not highlight
				if(selectedOptions.requiredChecked < selectedOptions.requiredOptions){
					highLightRequiredOptions(selectedOptions.unselectedTableName[0]);
				} else {
					if ($("#addToGlobalCart").attr("cur_stepnum") && !$(this).hasClass("updateItem")) {
						if(window.runCodeAfterClickNextStep)window.runCodeAfterClickNextStep(itemDeInfo);
						showNextStep();
						return;
					}
					var _selfOrderHasOneRequiredItem = false;
					var _onlineHasOneRequiredItem = false;
					var _stoHasOneRequiredItem = false;
					if($(this).hasClass("updateItem")){
						//if it is re-opened item from cart, update item by adding new and deleting old in cart
						var sortedCartIndex = $(this).attr("index");
						updateItemWithOptionToCart(sortedCartIndex, itemindex, selectedOptions, numNewItem, maxQuantRestriction, weight);
					} else {
						if(checkSelfOrderOneRquiredItem()) {
							_selfOrderHasOneRequiredItem = true;
						}
						else if(checkOnlineOneRquiredItem()) {
							_onlineHasOneRequiredItem = true;
						}
						else if(checkStoOneRquiredItem()) {
							_stoHasOneRequiredItem = true;
						}
						addItemWithOptionToCart(itemindex, selectedOptions, numNewItem, maxQuantRestriction, weight);
					}
					closeMenuDetails();	
					sortAndMergeMenuItems(context.cart);
					if(_selfOrderHasOneRequiredItem) {
						kioskCheckoutClicked();
					}
					else if(_onlineHasOneRequiredItem) {
						$('#readyOrder').click();
					}
					else if(_stoHasOneRequiredItem) {
						// readyDineinClicked();
						$('#readyOrder').click();
					}

					// if(itemDeInfo.menuitem.id == -1) {
					// 	$("#viewCart").click()
					// 	$('.loadingIconHolder_center').css('display', '');
					// 	setTimeout(() => {
					// 		$('.loadingIconHolder_center').css('display', 'none');
					// 		$('#readyOrder').click();
					// 	}, 500);
					// }
				}
			} else {
				var ctgynames = getCtgynameString(meetGroupedMinium);
				displayGroupedMinWarning(ctgynames, meetGroupedMinium.max)
				return;
			}
		}
	});		
}

function addToCartTapped(obj, disableHeaderMove){
	// 05/03/2018 check if the item has required option, if so pop up detail page
	var isRequired = false;
	var item_id = $(obj).attr("id");
	var menuItem = context.items[item_id];
	var options = JSON.parse(menuItem.options);
	if(options){
		var optCatNum = {};
		if (options && options.optCatNum)
		{
			optCatNum = options.optCatNum;
		}
		var opts = [];
		if (options && options.opts)
		{
			opts = options.opts;
		}

		//miyu: get linked menu item
		if (opts && opts.linkedWithDBID)
		{
			opts = getOptionsFromLinkedMenuItem(opts, context.items);
		}

		var todayDate = new Date();
		var todayMillisecond = new Date((todayDate.getMonth() + 1) + '/' + todayDate.getDate() + '/' + todayDate.getFullYear()).getTime();
		var currentTime = todayDate.getTime();
		var dayOfTheWeek = todayDate.getDay();
		var displayThisDish = true;
		var dishRestricted = false;
		var isSoldOut = checkIsSoldout(options, context.rest.name);
		if (options)
		{
			var availableDays = [];
			var availableTime = [];
			if (options.dishDate)
			{
				getAvailableDays(options, availableDays);
			}
			if (options.dishHour)
			{
				getAvailableTime(options, availableTime, todayMillisecond);
			}

			displayThisDish = checkAvailable(availableDays, availableTime, currentTime, dayOfTheWeek, options, todayMillisecond);

			if (options.maxQuantRestriction)
			{
				dishRestricted = checkRestricted(options.maxQuantRestriction, menuItem.id, 1);
			}
		}
		if (!displayThisDish)
		{
			displayUnavailableWarning(options.dishDate, options.dishHour, options.dishHourDate);
			return;
		}
		if (isSoldOut) {
			displaySoldOutWarning();
			return;
		}
		if (dishRestricted)
		{
			displayRestrictedWarning(options.maxQuantRestriction, menuItem.id);
			return;
		}

		//find required option
		if (opts && opts.length)
		{
			//find required option
			for(var i = 0; i < opts.length ; i++){
				if(opts[i].required){
					var isRequired = true;
					break;
				}
			}
		}
		if (optCatNum)
		{
			for (var key in optCatNum)
			{
				if (optCatNum.hasOwnProperty(key))
				{
					if (optCatNum[key].optCatReq)
					{
						isRequired = true;
						break;
					}
				}
			}
		}

		var autoOptionPopup = null;
		if (options)
		{
			autoOptionPopup = options.autoOptionPopup;
		}
		if (typeof autoOptionPopup != 'undefined' && autoOptionPopup != null)
		{
			if (autoOptionPopup != '0')
			{
				isRequired = true;
			}
		}
	}

	if(isRequired || options.pricePerUnit){
		//show details page
		var target = obj.closest(".menuitem-Container");
		disableHeaderMove = saveScrollandloadDetails(target, disableHeaderMove);		
	} else {
		_addItemToCart(obj, options); 
	}
}

function _addItemToCart(obj, options){
	if (options && options.maxQuantRestriction)
	{
		addItemToCart(obj, options.maxQuantRestriction);
	} else
	{
		addItemToCart(obj);
	}
}


function changeItemQtyTapped(itemDeInfo, increase){
	if(!$(this).hasClass('disable-btn')){
		if(increase){
			//increase
			itemDeInfo.itemcount = increaseItemCount(itemDeInfo.itemcount);	
		} else if (itemDeInfo.itemcount > 1){
			//decrease
			itemDeInfo.itemcount = decreaseItemCount(itemDeInfo.itemcount);
		}
		$("#item_details_page #itemCount").html("<a>" + itemDeInfo.itemcount + "</a>");	
		updateItemsTotalPriceInDetails(itemDeInfo.itemcount, itemDeInfo.curPrice);
	}
}

function changeOptionCountTapped(obj, itemDeInfo, increase){
	var needToUpdatePrice = 0.00;
	if(increase){
		needToUpdatePrice = optionCounterClicked(true, obj, itemDeInfo.optionPriceList);		
	} else {
		needToUpdatePrice = optionCounterClicked(false, obj, itemDeInfo.optionPriceList, itemDeInfo.originalPrice);	
	}
	if(needToUpdatePrice){
		itemDeInfo.curPrice = updateItemsPriceWithOptions(itemDeInfo.originalPrice, itemDeInfo.optionPriceList);
		updateItemsTotalPriceInDetails(itemDeInfo.itemcount, itemDeInfo.curPrice);
	}
}

function checkboxChanged(obj, itemDeInfo){
	checkboxInputChanged(obj, itemDeInfo.optionPriceList);
	itemDeInfo.curPrice = updateItemsPriceWithOptions(itemDeInfo.originalPrice, itemDeInfo.optionPriceList);
	updateItemsTotalPriceInDetails(itemDeInfo.itemcount, itemDeInfo.curPrice);
}

function radioboxChanged(obj, itemDeInfo){
	radioInputChanged(obj, itemDeInfo.optionPriceList);
	itemDeInfo.curPrice = updateItemsPriceWithOptions(itemDeInfo.originalPrice, itemDeInfo.optionPriceList);
	updateItemsTotalPriceInDetails(itemDeInfo.itemcount, itemDeInfo.curPrice);
}

//common except selforder 
function getOptionTableHolder(){
	return $('#item_details_page_container');
}

//Miyu: moved to common from selforder 
function bindSpecialInsEvent(itemDeInfo, groupedChoiceList){	
	$('#specialIns_input').unbind().on('click', function() {
		showKeyboardCB($(this));
		$("#item_details_page_filter").after('<div id="special_instructions_filter"></div>');
		$("#special_instructions_filter").unbind().tap(function () { 
			document.activeElement.blur(); // hide ipad keyboard
			$("#special_instructions_filter").remove();
		});
		$("#addToGlobalCart").addClass('disable-btn');  //disable add to cart btn 
		$("#increaseItemCount").addClass('disable-btn');  //diable plus and minus
		
		//tring to scroll to bottom when keyboard pops up
		//$('#item_details_page_container').scrollTop($(this).offset().top);
		
		//we need to unbind the .tap from the btn in order to prevent iPad to stuck in darkfilter page
		$("#addToGlobalCart").unbind();
		$("#increaseItemCount").unbind();
		$("#decreaseItemCount").unbind();
		$("#item_image").tap(function(e){
			e.preventDefault();
			return false;
		});
	});
	var is_safari_or_uiwebview = /(iPhone|iPod|iPad|Macintosh).*AppleWebKit/i.test(navigator.userAgent);
	// for mobile only since the above click event not working on mobile
	if(is_safari_or_uiwebview || navigator.userAgent.includes('Android')) {
		$('#specialIns_input').tap(function() {
			showKeyboardCB($(this));
		});
	}

	$('#specialIns_input').blur(function() {
		$("#special_instructions_filter").remove();
		if ($("#addToGlobalCart").attr("priceperunit") == null) {
			//enable 
			$("#addToGlobalCart").removeClass('disable-btn');
			$("#increaseItemCount").removeClass('disable-btn');
			$("#decreaseItemCount").removeClass('disable-btn');
			//re-bind 
		}
		onMenuDetailsLoaded(itemDeInfo, groupedChoiceList);
	});
}

function getRequiredMenuitems(type) {
	var requiredItemIds = {};
	for (var item of context.allitems) {
		try {
			var options = JSON.parse(item.options);
			if (options.requiredItemStr && options.requiredItemStr.indexOf(type) >= 0) {
				requiredItemIds[item.id] = 0;
			}
		} catch (e) { console.log(e) }
	}
	for (var item of context.cart.sortedItems) {
		if (requiredItemIds.hasOwnProperty(item.id)) {
			requiredItemIds[item.id] = 1;
		}
	}
	var requiredMenuitems = [];
	for (var id in requiredItemIds) {
		if (requiredItemIds[id] == 0) {
			requiredMenuitems.push(id);
		}
	}
	return requiredMenuitems;
}

function getGlobalOptions(menuItem) {
	var options;
	if (typeof dishGlobalOptions !== "undefined" && dishGlobalOptions != "") {
		try {
			options = JSON.parse(dishGlobalOptions);
		} catch (e) {
			// if it's already an object/array, fall back directly
			options = dishGlobalOptions;
		}
		// Filter out any option objects explicitly marked with uv === false
		if (Array.isArray(options)) {
			options = options.filter(function (opt) { return opt == null || opt.uv === true; });
			// Mark all global options to be pre-collapsed by default if UI supports it
			options.forEach(function (opt) {
				if (opt && typeof opt.preCollapsed === 'undefined') opt.preCollapsed = true;
			});
		}
	}
	if (window.cusgetGlobalOptionsCallback) {
		options = window.cusgetGlobalOptionsCallback(options, menuItem);
	}
	return options;
}

function toLocaleDateStringByGivenDate(d) {
	return '' + (d.getMonth() + 1) + "/" + d.getDate() + "/" + d.getFullYear()
}
var weekDayMapping = {
	1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri", 6: "sat", 0: "sun"
}
var canPlaceOrder = true;
function checkRestOpening() {
	canPlaceOrder = true;
	var lNowTimestamp = new Date().getTime();
	var lArrOpenHours = [];
	var lastEnd;
	try {
		var openHours = context.rest.openHours[weekDayMapping[new Date().getDay()]];
		if (openHours.start == "close")
		{
			canPlaceOrder = false;
			return false;
		}
		var lStr = openHours.str;
		if (!lStr) return true; // default to true
		lArrOpenHours = JSON.parse(lStr); // ["11:00-15:00", "18:00-22:00"]
	} catch (e) { }
	for (var lTime of lArrOpenHours) {
		//"11:00-15:00"
		var lTemp = lTime.split("-");
		var lTimeStart = new Date(toLocaleDateStringByGivenDate(new Date()) + " " + lTemp[0]).getTime();
		var lTimeEnd = new Date(toLocaleDateStringByGivenDate(new Date()) + " " + lTemp[1]).getTime();

		if (window.isOnline && window.enableLastMinsSubmitOrderCheck && context.rest && context.rest.config && (context.rest.config.hidingOnlineOrderDisplayOnMinsBefore || context.rest.config.printImmediatelyWithMins)) {
			// for online order, consider the cooking time
			var val1 = parseInt(context.rest.config.printImmediatelyWithMins) || 0;
			var val2 = parseInt(context.rest.config.hidingOnlineOrderDisplayOnMinsBefore) || 0;
			var bufferMins = Math.max(val1, val2);
			if (!isNaN(bufferMins) && bufferMins > 0) {
				lTimeEnd -= bufferMins * 60 * 1000;
			}
		}

		lastEnd = lTimeEnd;
		if (lTimeStart <= lNowTimestamp && lNowTimestamp < lTimeEnd) {
			return true;
		}
	}
	if (lNowTimestamp > lastEnd) {
		canPlaceOrder = false;
	}
	return false;
}