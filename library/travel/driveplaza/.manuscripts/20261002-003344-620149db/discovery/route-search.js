var keiyuMapDeleteNum = 99;
var isKeiyuDeleteLinkClick = false;
var cntKeiyuMap = 0; // マップに設定されている経由地数
var isOnMapKeiyu1 = false; // マップ上の経由地1が設定されているか
var isOnMapKeiyu2 = false; // マップ上の経由地2が設定されているか
var isOnMapKeiyu3 = false; // マップ上の経由地3が設定されているか
// V12.0.0 ADD START
var isOnMapKeiyu4 = false; // マップ上の経由地4が設定されているか
var isOnMapKeiyu5 = false; // マップ上の経由地5が設定されているか
// V12.0.0 ADD END

var isFollowBtn = false; //追従 再検索ボタン押下時


var drLangCode = "JP"; //設定言語

var constDpSearchTop_List={
"JP":"/dp/SearchTop",
"EN":"/dp/SearchTopEN",
"CN":"/dp/SearchTopCN",
"KR":"/dp/SearchTopKR",
"TW":"/dp/SearchTopTW",
"TH":"/dp/SearchTopTH"
};


var constDpSearchQuick_List={
"JP":"/dp/SearchQuick",
"EN":"/dp/SearchQuickEN",
"CN":"/dp/SearchQuickCN",
"KR":"/dp/SearchQuickKR",
"TW":"/dp/SearchQuickTW",
"TH":"/dp/SearchQuickTH"
};

var constDpPrint_List={
"JP":"/dp/RoutePrintNew",
"EN":"/dp/RoutePrintNewEN",
"CN":"/dp/RoutePrintNewCN",
"KR":"/dp/RoutePrintNewKR",
"TW":"/dp/RoutePrintNewTW",
"TH":"/dp/RoutePrintNewTH"
};

var constCurrentLocationText_List={
"JP":"現在地",
"EN":"Search here",
"CN":"搜索",
"KR":"이곳을 검색하기",
"TW":"查詢",
"TH":"ค้นหาที่นี่"
};

var constCurrentLocationErrMsg_List={
"JP":"現在地が取得できませんでした。",
"EN":"Location error",
"CN":"Location error",
"KR":"Location error",
"TW":"Location error",
"TH":"Location error"
};

var constKeywordSearchErrMsg_List={
"JP":"キーワードに該当する出発地・到着地が見つかりませんでした。",
"EN":"Keyword search error",
"CN":"Keyword search error",
"KR":"Keyword search error",
"TW":"Keyword search error",
"TH":"Keyword search error"
};

var constLogSearchErrMsg_List={
"JP":"検索履歴がありません。(保存期間は３か月となっております)",
"EN":"No search history. (Stored for three months.)",
"CN":"无检索履历（履历保存期限为3个月）",
"KR":"검색 이력이 없습니다.(저장 기간은 3개월입니다)",
"TW":"無查詢紀錄。（保存期限為三個月）",
"TH":"ไม่มีประวัติการค้นหา (ระยะเวลาการเก็บรักษาข้อมูล 3 เดือน)"
};

var constLogSearchDeleteMsg_List={
"JP":"この履歴を削除",
"EN":"Delete this history",
"CN":"删除这条纪录",
"KR":"이 이력을 삭제",
"TW":"刪除這條紀錄",
"TH":"ลบทิ้งประวัตินี้"
};

var constLogSearchViaText_List={
"JP":"経由",
"EN":"via",
"CN":"via",
"KR":"via",
"TW":"via",
"TH":"via"
};

var constWayPointDeleteMsg_List={
"JP":"この経由地を削除",
"EN":"Delete this transit point",
"CN":"删除此经由地",
"KR":"이 경유지 삭제하기",
"TW":"刪除此行經地",
"TH":"ลบจุดเปลี่ยนนี้"
};

var constSearchIcInputMsg_List={
"JP":'IC名を<br class="is-showsp">調べて入力',
"EN":'Search for and input <br class="is-showsp">an IC name',
"CN":'查出IC名后输入',
"KR":'IC명을 조사하여 입력',
"TW":'查出IC名後輸入',
"TH":'ค้นหาชื่อ  IC แล้วป้อนข้อมูล'
};


var constUdClass_UP_List={
"JP":'上り線',
"EN":'Inbound',
"CN":'Inbound',
"KR":'Inbound',
"TW":'Inbound',
"TH":'Inbound'
};

var constUdClass_DOWN_List={
"JP":'下り線',
"EN":'Outbound',
"CN":'Outbound',
"KR":'Outbound',
"TW":'Outbound',
"TH":'Outbound'
};

var constUdClass_WEST_List={
"JP":'西行き',
"EN":'Westbound',
"CN":'Westbound',
"KR":'Westbound',
"TW":'Westbound',
"TH":'Westbound'
};

var constUdClass_EAST_List={
"JP":'東行き',
"EN":'Eastbound',
"CN":'Eastbound',
"KR":'Eastbound',
"TW":'Eastbound',
"TH":'Eastbound'
};

var constUdClass_IN_List={
"JP":'内回り',
"EN":'Inner loop',
"CN":'Inner loop',
"KR":'Inner loop',
"TW":'Inner loop',
"TH":'Inner loop'
};

var constUdClass_OUT_List={
"JP":'外回り',
"EN":'Outer loop',
"CN":'Outer loop',
"KR":'Outer loop',
"TW":'Outer loop',
"TH":'Outer loop'
};

var constModalInputTitle_dep_List={
"JP":'出発IC名を調べて入力する',
"EN":'Departure Input interchange',
"CN":'输入起点IC',
"KR":'출발 IC 입력',
"TW":'輸入出發交流道',
"TH":'ใส่ชื่อ IC เริ่มต้น'
};

var constModalInputTitle_arr_List={
"JP":'到着IC名を調べて入力する',
"EN":'Arrival Input interchange',
"CN":'输入终点IC',
"KR":'도착 IC 입력',
"TW":'輸入抵達交流道',
"TH":'ใส่ชื่อ IC ปลายทาง'
};

var constModalInputTitle_via_List={
"JP":'経由地IC名を調べて入力する',
"EN":'Transit points Input interchange',
"CN":'输入经由点IC',
"KR":'경유 IC 입력',
"TW":'輸入行經點交流道',
"TH":'ใส่ชื่อ IC ทางผ่าน'
};

/*
var constStartIcEmptyErrMsg_List={
"JP":'出発IC名読みはかならず入力してください。',
"EN":'You must enter the reading for your Departure IC name.',
"CN":'必须输入起点IC的名字读法。',
"KR":'출발 IC명은 반드시 입력해 주십시오.',
"TW":'請務必輸入出發交流道的名稱。',
"TH":'กรุณาใส่การออกเสียงชื่อ IC เริ่มต้น'
};

var constArriveIcEmptyErrMsg_List={
"JP":'到着IC名読みはかならず入力してください。',
"EN":'You must enter the reading for your Arrival IC name.',
"CN":'必须输入终点IC的名字读法。',
"KR":'도착 IC명은 반드시 입력해 주십시오.',
"TW":'請務必輸入抵達交流道的名稱。',
"TH":'กรุณาใส่การออกเสียงชื่อ IC ปลายทาง'
};
*/

var constLNErrMsg_List1={
"JP":"走行距離を入力してください。",
"EN":"Enter the driving distance.",
"CN":"请输入里程。",
"KR":"주행 거리를 입력해주세요.",
"TW":"請輸入里程。",
"TH":"กรอกระยะทางที่ขับขี่"
};

var constLNErrMsg_List2={
"JP":"走行距離には半角数値を入力してください。",
"EN":"Enter one-corner figures for mileage.",
"CN":"请输入半角数值作为行驶距离。",
"KR":"주행 거리에는 반자 값을 입력해주세요.",
"TW":"請輸入半角數值作為行駛距離。",
"TH":"กรอกตัวเลขมุมหนึ่งสำหรับระยะทาง"
};

var constLNErrMsg_List3={
"JP":"走行距離には小数点1桁までを入力してください。",
"EN":"Enter up to one decimal digit for mileage.",
"CN":"在行驶距离上输入小数点至1位。",
"KR":"주행 거리는 소수점한 자리까지 입력해야 합니다.",
"TW":"在行駛距離上輸入小數點至1位。",
"TH":"กรอกระยะทางทศนิยมสูงสุดหนึ่งหลัก"
};

var constLNErrMsg_List4={
"JP":"走行距離には正の数値を入力してください。",
"EN":"Enter a positive value for the driving distance.",
"CN":"请输入正数作为里程。",
"KR":"주행 거리에 양수 값을 입력하십시오.",
"TW":"請輸入正數作為里程。",
"TH":"กรอกค่าบวกสำหรับระยะทางการขับขี่"
};

var constLNErrMsg_List5={
"JP":"入力された走行距離は総距離を超えています。",
"EN":"The entered trip distance exceeds the total distance.",
"CN":"输入的里程超过总里程。",
"KR":"입력한 마일리지가 총 거리를 초과합니다.",
"TW":"輸入的里程超過總里程。",
"TH":"ระยะทางขับขี่ที่ป้อนเกินระยะทางรวม"
};

var constLNErrMsg_List6={
"JP":"走行距離の合計が0より大きい数値となるように入力してください。",
"EN":"Enter so that the total mileage is greater than 0.",
"CN":"请输入总里程大于0的数字。",
"KR":"주행거리의 합계가 0보다 큰 수치가 되도록 입력해주세요.",
"TW":"請輸入總里程大于0的數字。",
"TH":"ใส่ให้ระยะทางรวมมากกว่า 0."
};


var tmpIcCode = "";

//----------------------
// ページロード後の処理
//----------------------
$(document).ready(function(){
//jQuery(document).ready(function($){
/*
	// IE8以下用にindexOf関数を定義
	if(!Array.indexOf){
		Array.prototype.indexOf = function(o)
  		{
    		for(var i in this){
      			if(this[i] == o){
        			return i;
     	 		}
    		}
    		return -1;
  		}
	}
*/
/*
	$('input[name="startPlaceKana"]').attr('placeholder', '例) 港北');
	$('input[name="arrivePlaceKana"]').attr('placeholder', '例) 仙台南');
	$('input[name="keiyuPlaceKana"]').attr('placeholder', '例) 港北');
	$('input[name="keiyuPlaceKana2"]').attr('placeholder', '例) 港北');
	$('input[name="keiyuPlaceKana3"]').attr('placeholder', '例) 港北');
*/

	
	// 言語別メッセージ設定
	setLangMessage();

/*
	// 多言語の出発地・到着地emptyエラーメッセージ
	if(drLangCode != "JP"){
		if($('#strutsErrArea').length){
			var errMsg1 = $('#strutsErrArea div:first').text();
			if(errMsg1 == constStartIcEmptyErrMsg_List["JP"]){
				$('#strutsErrArea div:first').text(constStartIcEmptyErrMsg_List[drLangCode]);
			}else if(errMsg1 == constArriveIcEmptyErrMsg_List["JP"]){
				$('#strutsErrArea div:first').text(constArriveIcEmptyErrMsg_List[drLangCode]);
			}
			var errMsg2 = $('#strutsErrArea div:last').text();
			if(errMsg2 == constStartIcEmptyErrMsg_List["JP"]){
				$('#strutsErrArea div:last').text(constStartIcEmptyErrMsg_List[drLangCode]);
			}else if(errMsg2 == constArriveIcEmptyErrMsg_List["JP"]){
				$('#strutsErrArea div:last').text(constArriveIcEmptyErrMsg_List[drLangCode]);
			}
		}
	}
*/

	// 経由地があった場合に入力エリア表示
	var keiyuItems = [];
	if($('input[name="keiyuPlaceKana"]').val()){
		$('.js-addWaypoint').click();
		keiyuItems.push($('input[name="keiyuPlaceKana"]').val());
	}
	if($('input[name="keiyuPlaceKana2"]').val()){
		$('.js-addWaypoint').click();
		keiyuItems.push($('input[name="keiyuPlaceKana2"]').val());
	}
	if($('input[name="keiyuPlaceKana3"]').val()){
		$('.js-addWaypoint').click();
		keiyuItems.push($('input[name="keiyuPlaceKana3"]').val());
	}
	// V12.0.0 ADD START
	if($('input[name="keiyuPlaceKana4"]').val()){
		$('.js-addWaypoint').click();
		keiyuItems.push($('input[name="keiyuPlaceKana4"]').val());
	}
	if($('input[name="keiyuPlaceKana5"]').val()){
		$('.js-addWaypoint').click();
		keiyuItems.push($('input[name="keiyuPlaceKana5"]').val());
	}
	// V12.0.0 ADD END
	var i = 0;
	$('.keiyuPlaceClass').each(function() {
        	$(this).val(keiyuItems[i]);
		i++;
    	});

	// 検索前チェック
	$('form[name="routesearchForm"]').on('submit', function () {
	//$('form[name="routesearchForm"]').submit(function(){
		// 現在地設定確認
		return chkCurrentLocation();
	});

	// 検索条件画面ならリターン
	if(selectedKeiroNumber == 99){
		return;
	}

	// ルートマップ設定
	$('#iframeRouteMap').load(function(){
		document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
		document.getElementById("iframeRouteMap").contentWindow.traceData1 = traceData1;
		document.getElementById("iframeRouteMap").contentWindow.traceData2 = traceData2;
		document.getElementById("iframeRouteMap").contentWindow.traceData3 = traceData3;
		document.getElementById("iframeRouteMap").contentWindow.traceData4 = traceData4;
		document.getElementById("iframeRouteMap").contentWindow.traceData5 = traceData5;

		// ルートマップ初期化
		init();

	});


	// 1時間前後ボタンイベント設定
	//$('a.js_one_ago, a.js_one_later').click(function () {
	$('a.js_one_ago, a.js_one_later').on('click', function () {
		// hrefリンク無効化
		return false;
  	});
	$('.js_one_ago, .js_one_later').on('click', function () {
		// 現在地設定確認
		if( !chkCurrentLocation()){
			return false;
		}

	var form = $('#js-departureICCal').closest('form')[0],
	year = form.searchYear.value,
     	month = form.searchMonth.value - 1,
        day = form.searchDay.value,
        hour = form.searchHour.value,
        minute = form.searchMinute.value,
        time = moment(new Date(year, month, day, hour, minute));
	time.add('h', $(this).data('hour'));
 	form.searchYear.value = time.get('year');
    	form.searchMonth.value = time.get('month') + 1;
    	form.searchDay.value = time.get('date');
    	form.searchHour.value = time.get('hour');
    	form.searchMinute.value = time.get('minute');
	// 20200622 ADD
	form.action = form.action + "#priceResult";

	form.submit();
	return false;
	});

	// SAPAサービスアイコン設定
	getSAPAServiceIcon();

	// 道路から選ぶhtml取得
	getRoadSearchHtml();
	if (winResizeWidth >= 768) {
		// 検索結果画面かつPCなら道路から選ぶを初期選択に
		//$('#js_search_address_li').addClass('is-current');
		//$('#js_search_address').addClass('is-current');
	}


	// シームレス料金表示時はメッセージを表示する。
    	if($(".seamlessZeroArea_keiro" + selectedKeiroNumber).is(":hidden") || $(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).is(":hidden")){
        	$(".seamlessMsgArea").show();
    	}else{
        	$(".seamlessMsgArea").hide();
  	}

	// 現在地検索された場合の対応
	if(selectickindflg == "1"){ // 出発地に現在地文言設定
		$('.js-departureICField').val(constCurrentLocationText_List[drLangCode]);
		$('.js-departureICField_follow').val(constCurrentLocationText_List[drLangCode]);
	}else if(selectickindflg == "2"){ // 到着地に現在地文言設定
		$('.js-arrivalICField').val(constCurrentLocationText_List[drLangCode]);
		$('.js-arrivalICField_follow').val(constCurrentLocationText_List[drLangCode]);
	}else if(selectickindflg == "3"){ // 出発地,到着地に現在地文言設定
		$('.js-departureICField').val(constCurrentLocationText_List[drLangCode]);
		$('.js-arrivalICField').val(constCurrentLocationText_List[drLangCode]);
		$('.js-departureICField_follow').val(constCurrentLocationText_List[drLangCode]);
		$('.js-arrivalICField_follow').val(constCurrentLocationText_List[drLangCode]);
	}

	// URLのSearchパラメータ取得（マイルートから遷移してきた場合）
	if( window.location.pathname != null && window.location.pathname.indexOf("SearchOutside") != -1 && 1 < window.location.search.length){
		var searchParam = {},
		    names = ['startPlace', 'arrivePlace', 'keiyuPlace'];
		searchParam = getUrlSearchString();
		
		for (var i = 0; i < names.length; i++) {
			var tempIcCode = searchParam[names[i] + "Code"];
			if(names[i] == "keiyuPlace"){
				// V12.0.0 MOD
				//var keiyuNo = ['', '2', '3'], keiyuText = "";
				var keiyuNo = ['', '2', '3', '4', '5'], keiyuText = "";
				for(var j = 0; j <= keiyuNo.length; j++){
					var tempIcCodeKeiyu = searchParam[names[i] + "Code" + keiyuNo[j]];
					if(tempIcCodeKeiyu){
						var tmpKeiyuIcName = searchIcFromCode(tempIcCodeKeiyu);
						
						$('input[name="' + names[i] + 'Kana' + keiyuNo[j] + '"]').val(tmpKeiyuIcName);
						if(keiyuText == ""){
							keiyuText +=  tmpKeiyuIcName;
						}else{
    							keiyuText += "、" + tmpKeiyuIcName;
						}

						$('.js-addWaypoint').click();
						$('.keiyuPlaceClass').eq(j).val(tmpKeiyuIcName);

					}
				}
				if(keiyuText != "") $('#keiyuPlaceArea').text(keiyuText);
			} else {
				if(tempIcCode){
					var tmpIcName = searchIcFromCode(tempIcCode);
					//$(':hidden[name="' + names[i] + '"]').val(tmpIcName);
					//$('input[name="' + names[i] + 'Kana"]').val(tmpIcName).addClass('js_highlight_bg');
					$('#' + names[i] + 'Area').text(tmpIcName);
					$('input[name="' + names[i] + 'Kana"]').val(tmpIcName);
					
				}
			}
		}
	}

	// 検索結果の一般道の場合はアコーディオンを開く
	$(".general-road").each(function(i, o){
     		var elm = $(this).closest('dd.js-acContent');
		var elm1 = elm.prev('dt.js-acTitle');
		elm1.click();
    	});
	
	// 深夜割引料金シミュレーションボタンイベント設定
	$('.ltInArea .js_lt_submit').on('click', function () {

		var keiroareaid = $(this).data('keiroareaid');

		searchLatenight(keiroareaid);

		$(".ltInArea.keiroAreaId_" + keiroareaid).hide();
		$(".ltOutArea.keiroAreaId_" + keiroareaid).show();

		$(".scroll-area").scrollTop(0);

		return false;
  	});
	$('.ltOutArea .js_lt_back').on('click', function () {

		var keiroareaid = $(this).data('keiroareaid');

		$(".ltInArea.keiroAreaId_" + keiroareaid).show();
		$(".ltOutArea.keiroAreaId_" + keiroareaid).hide();

		$(".scroll-area").scrollTop(0);

		return false;
  	});
	// 複数の深夜時間帯をまたぐご利用イベント設定
	$('.ltInArea .lt_chk_fukusu').on('click', function() {

		var inputid = $(this).data('inputid');

		if($(this).prop('checked')){
			$("#" + inputid).show();
			$("#onemark_" + inputid).show();
		}else{
			$("#" + inputid + " input[type=text]").val("");
			$("#" + inputid).hide();
			$("#onemark_" + inputid).hide();
		}
	});
	// 再検索エリア遷移リンクイベント設定
	$('.ltInArea .js_lt_research').on('click', function() {

		//$('.js-modalclose').click();

		//return false;
	});
	// シミュレーションはこちらボタンイベント設定
	$('.js-simbtn').on('click', function() {
		var etcno = $(this).data('etcno');

		// 2024/10/25 ADD
		if(drLangCode == "JP"){
			if($(this).closest("dd.js-acContent").find("ul.js-tabmenu li:contains('ETC2.0')").hasClass('is-current')){
				//console.log("ETC2.0");
				etcno = "2";
			}
		}
		

		var keiroareaid = $(this).data('keiroareaid');
		$("#select_etcno_" + keiroareaid).text(etcno);

		// 2024/10/9 ADD 説明アコーディオンを閉じておく
		if($("#latenight_notes_" + keiroareaid).hasClass('is-open')){
			$("#latenight_noteslink_" + keiroareaid).click();
		}

		$(".ltInArea.keiroAreaId_" + keiroareaid).show();
		$(".ltOutArea.keiroAreaId_" + keiroareaid).hide();

		$(".scroll-area").scrollTop(0);
	});
	
	// 2024/8/19 ADD
	// 複数の深夜時間帯をまたぐご利用イベント設定
	$('.ltInArea .lt_chk_ichiritsu').on('click', function() {

		var inputid = $(this).data('inputid');
		var dstroad = $(this).data('dstroad');

		if($(this).prop('checked')){
			$("#" + inputid + " input[type=text]").val(dstroad);
		}else{
			$("#" + inputid + " input[type=text]").val("0");
		}
	});

	// 深夜割引シミュレーションの場合は全区間アコーディオンを開く
	if($('input[name="latenight"]').val() == "on"){
		$(".js-accordionAllOpen").each(function(i, o){
     			if(i % 2 == 0){
			//if(i == 0){
				$(this).click();
			}
    		});
	}
});


// 言語別メッセージ設定
function setLangMessage(){

	if( window.location.pathname != null ){
		if( window.location.pathname.indexOf("EN") != -1 ){
			drLangCode = "EN";
		}else if( window.location.pathname.indexOf("CN") != -1 ){
			drLangCode = "CN";
		}else if( window.location.pathname.indexOf("KR") != -1 ){
			drLangCode = "KR";
		}else if( window.location.pathname.indexOf("TW") != -1 ){
			drLangCode = "TW";
		}else if( window.location.pathname.indexOf("TH") != -1 ){
			drLangCode = "TH";
		}
	}

}

// 現在地チェック
function chkCurrentLocation(){
		var isStartIC = false;
		var isArriveIC = false;
		
if(!isFollowBtn){
		if($('.js-departureICField').val() == constCurrentLocationText_List[drLangCode]){
			if(!isGeolocation){
				alert(constCurrentLocationErrMsg_List[drLangCode]);
				return false;
			}
			$('.js-departureICField').val(nearestIC);
			isStartIC = true;
		}
		if($('.js-arrivalICField').val() == constCurrentLocationText_List[drLangCode]){
			if(!isGeolocation){
				alert(constCurrentLocationErrMsg_List[drLangCode]);
				return false;
			}
			$('.js-arrivalICField').val(nearestIC);
			isArriveIC = true;
		}
}else{
		// action先が検索トップ画面なら処理しない
		if($('#formSearch_follow').attr('action') == constDpSearchTop_List[drLangCode]){
			return true;
		}

		if($('.js-departureICField_follow').val() == constCurrentLocationText_List[drLangCode]){
			if(!isGeolocation){
				alert(constCurrentLocationErrMsg_List[drLangCode]);
				return false;
			}
			$('.js-departureICField_follow').val(nearestIC);
			isStartIC = true;
		}
		if($('.js-arrivalICField_follow').val() == constCurrentLocationText_List[drLangCode]){
			if(!isGeolocation){
				alert(constCurrentLocationErrMsg_List[drLangCode]);
				return false;
			}
			$('.js-arrivalICField_follow').val(nearestIC);
			isArriveIC = true;
		}
}
		if(isStartIC && !isArriveIC){
			$('input[name="selectickindflg"]').val("1");
		}else if(!isStartIC && isArriveIC){
			$('input[name="selectickindflg"]').val("2");
		}else if(isStartIC && isArriveIC){
			$('input[name="selectickindflg"]').val("3");
		}else{
			$('input[name="selectickindflg"]').val("0");
		}

	return true;
}

// --- 地図から探す ---
function onStartIC(id, name){

	// 検索結果画面なら
	if(selectedKeiroNumber != 99){
		var form = $('#js-departureICCal').closest('form')[0];
		if(id == 0){
		form.startPlaceKana.value = "";
		} else {
		form.startPlaceKana.value = name;
		}
		return false;
	}

	if(id == 0){
		document.routesearchForm.startPlaceKana.value = "";
	} else {
		document.routesearchForm.startPlaceKana.value = name;
	}
/*
	if($('body').hasClass('breakpoint-1')){
		$('.is-open').stop().slideToggle().toggleClass('is-open');
	}else{
		$('.is-open').stop().fadeToggle().toggleClass('is-open');
	}
	unpinBackgroundGray();
*/
	return false;
}



function onArrivalIC(id, name){

	// 検索結果画面ならリターン
	if(selectedKeiroNumber != 99){
		var form = $('#js-departureICCal').closest('form')[0];
		if(id == 0){
		form.arrivePlaceKana.value = "";
		} else {
		form.arrivePlaceKana.value = name;
		}
		return false;
	}

	if(id == 0){
		document.routesearchForm.arrivePlaceKana.value = "";
	} else {
		document.routesearchForm.arrivePlaceKana.value = name;
	}
/*
	if($('body').hasClass('breakpoint-1')){
		$('.is-open').stop().slideToggle().toggleClass('is-open');
	}else{
		$('.is-open').stop().fadeToggle().toggleClass('is-open');
	}
	unpinBackgroundGray();
*/
	return false;

}

function onThroughIC(id, name, num){

	// 検索結果画面ならリターン
	if(selectedKeiroNumber != 99){
		//return;
	}

	var form = $('#js-departureICCal').closest('form')[0];

	if(id == 0){
		// V12.0.0 MOD
		// 経由地数設定
		if(num == 0){
			isOnMapKeiyu1 = false;
		}else if(num == 1){
			isOnMapKeiyu2 = false;
		}else if(num == 2){
			isOnMapKeiyu3 = false;
		}else if(num == 3){
			isOnMapKeiyu4 = false;
		}else if(num == 4){
			isOnMapKeiyu5 = false;
		}

		/*
		if(isOnMapKeiyu1 && isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 3;
		}else if(isOnMapKeiyu1 && isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(isOnMapKeiyu1 && !isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}else if(!isOnMapKeiyu1 && !isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 0;
		}else if(isOnMapKeiyu1 && !isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(!isOnMapKeiyu1 && isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(!isOnMapKeiyu1 && !isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}else if(!isOnMapKeiyu1 && isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}
		*/

		//var list = ['', '2', '3'];
		var list = ['', '2', '3', '4', '5'];
		//document.routesearchForm['keiyuPlace' + 'Kana' + list[num]].value = "";
		form['keiyuPlace' + 'Kana' + list[num]].value = "";

		//if(!isKeiyuDeleteLinkClick){

		// 経由地入力エリアを削除
		// 無限ループ対策のフラグ
		if(keiyuMapDeleteNum != num){
			keiyuMapDeleteNum = num;
			//keiyuMapDeleteNum = 99;
			// 現在何も経由地入力エリアが表示されていない場合はスルー
			if($('.keiyuPlaceClass').length){
				
				// 表示されている数がidより少ない時は表示されている最も大きいidを削除
				if($('.keiyuPlaceClass').length <= num){
					num = $('.keiyuPlaceClass').length -1;
				}
				$('.js-removeWaypoint').eq(num).click();
				
			}
		}

		//}else{
		//	isKeiyuDeleteLinkClick = false;
		//}
	} else {
		// V12.0.0 MOD
		// 経由地数設定
		if(num == 0){
			isOnMapKeiyu1 = true;
		}else if(num == 1){
			isOnMapKeiyu2 = true;
		}else if(num == 2){
			isOnMapKeiyu3 = true;
		}else if(num == 3){
			isOnMapKeiyu4 = true;
		}else if(num == 4){
			isOnMapKeiyu5 = true;
		}

		/*
		if(isOnMapKeiyu1 && isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 3;
		}else if(isOnMapKeiyu1 && isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(isOnMapKeiyu1 && !isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}else if(!isOnMapKeiyu1 && !isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 0;
		}else if(isOnMapKeiyu1 && !isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(!isOnMapKeiyu1 && isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 2;
		}else if(!isOnMapKeiyu1 && !isOnMapKeiyu2 && isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}else if(!isOnMapKeiyu1 && isOnMapKeiyu2 && !isOnMapKeiyu3){
			cntKeiyuMap = 1;
		}
		*/
		var cntKeiyuMapTmp = 0;
		if(isOnMapKeiyu1){
			cntKeiyuMapTmp++;
		}
		if(isOnMapKeiyu2){
			cntKeiyuMapTmp++;
		}
		if(isOnMapKeiyu3){
			cntKeiyuMapTmp++;
		}
		if(isOnMapKeiyu4){
			cntKeiyuMapTmp++;
		}
		if(isOnMapKeiyu5){
			cntKeiyuMapTmp++;
		}
		cntKeiyuMap = cntKeiyuMapTmp;


		//var list = ['', '2', '3'];
		var list = ['', '2', '3', '4', '5'];
		//document.routesearchForm['keiyuPlace' + 'Kana' + list[num]].value = name;
		form['keiyuPlace' + 'Kana' + list[num]].value = name;


		// 現在何も経由地入力エリアが表示されていない場合
		if(!$('.keiyuPlaceClass').length){
			$('.js-addWaypoint').click();
		}else if($('.keiyuPlaceClass').length < cntKeiyuMap ){ // 表示されている経由地入力エリアの数が足りなければ追加
			$('.js-addWaypoint').click();
		}
		var i = 0;
		$('.keiyuPlaceClass').each(function() {
			//空のエリアがあればそこに入れる
			if(!$(this).val()){
				$(this).val(name);
				return false;
			}else{ //なければ
				if(i == num){
				$(this).val(name);
				return false;
				}
			}
        		i++;
    		});
		

		keiyuMapDeleteNum = 99;

	}

/*
	if($('body').hasClass('breakpoint-1')){
		$('.is-open').stop().slideToggle().toggleClass('is-open');
	}else{
		$('.is-open').stop().fadeToggle().toggleClass('is-open');				
	}
	unpinBackgroundGray();
*/
	return false;
}


// --- 道路から探す ---
var iAreaIndex = 0;
var iRoadIndex = 0;

// 対象インデックスの道路リスト消去
function clearRoadOne(p_index) {
	// DIVを隠す
	// 対象のvalueを返却
	return hidDiv("areaType"+p_index);
}

// 対象インデックスのICリスト消去
function clearIcOne(p_index) {
	// DIVを隠す
	// 対象のvalueを返却
	return hidDiv("roadType"+p_index);
}

// 道路表示切り替え関数
function chgRoad(id) {
	// 道路リスト消去関数呼び出し
	clearRoadOne(iAreaIndex);
	// ICリスト消去関数呼び出し
	clearIcOne(iRoadIndex);
	// 選択された地方名のDIVのみ表示
	viwDiv("areaType"+id);
	iAreaIndex=id;
}

// IC表示切り替え関数
function chgIc(id) {
	// ICリスト消去関数呼び出し
	clearIcOne(iRoadIndex);
	// 選択された道路名のDIVのみ表示
	viwDiv("roadType"+id);
	iRoadIndex=id;
}

// DIVタグ非表示関数
function hidDiv(div) {
	var i=0;
	for (i=0;i<getElementsByClassName(div).length;i++) {
		getElementsByClassName(div)[i].style.display = "none";
		getElementsByClassName(div)[i].style.visibility = "hidden";;
	}
}

// DIVタグ表示関数
function viwDiv(div) {
	var i=0;
	for (i=0;i<getElementsByClassName(div).length;i++) {
		getElementsByClassName(div)[i].style.display = "inline";
		getElementsByClassName(div)[i].style.visibility = "visible";
		getElementsByClassName(div)[i].style.position = "static";
	}
}

// 対象クラス名のコレクション取得
function getElementsByClassName(className){
    var collect = new Array();
    var objects = document.getElementsByTagName("div");
    var length = objects.length;
    for(i = 0; i < length; ++i){
        if(objects[i].className == className){
            collect.push(objects[i]);
        }
    }
    return collect;
}


//----------------------------------------------
//検索実行
//----------------------------------------------
function fSubmit(){
	isFollowBtn = false;

	setHiddenDate("js-departureICCal");

	$('input[name="keiyuPlaceKana"]').val("");
	$('input[name="keiyuPlaceKana2"]').val("");
	$('input[name="keiyuPlaceKana3"]').val("");
	// V12.0.0 ADD START
	$('input[name="keiyuPlaceKana4"]').val("");
	$('input[name="keiyuPlaceKana5"]').val("");
	// V12.0.0 ADD END
	var i = 0;
	// 経由地取得
	//$('.waypoint').children('input[type=text]').each(function() {
        $('.keiyuPlaceClass').each(function() {
        	var r = $(this).val();
		if(i==0){
			$('input[name="keiyuPlaceKana"]').val(r);
		}else if(i==1){
			$('input[name="keiyuPlaceKana2"]').val(r);
		}else if(i==2){
			$('input[name="keiyuPlaceKana3"]').val(r);
		}else if(i==3){
			$('input[name="keiyuPlaceKana4"]').val(r);
		}else if(i==4){
			$('input[name="keiyuPlaceKana5"]').val(r);
		}
		i++;
    	});

	//document.routesearchForm.submit();
}

//----------------------------------------------
//表示順変更時の検索実行
//----------------------------------------------
function fPrioritySubmit(id){
	isFollowBtn = false;
	setHiddenDate("js-departureICCal");

	$('input[name="keiyuPlaceKana"]').val("");
	$('input[name="keiyuPlaceKana2"]').val("");
	$('input[name="keiyuPlaceKana3"]').val("");
	// V12.0.0 ADD START
	$('input[name="keiyuPlaceKana4"]').val("");
	$('input[name="keiyuPlaceKana5"]').val("");
	// V12.0.0 ADD END
	var i = 0;
	// 経由地取得
	//$('.waypoint').children('input[type=text]').each(function() {
        $('.keiyuPlaceClass').each(function() {
        	var r = $(this).val();
		if(i==0){
			$('input[name="keiyuPlaceKana"]').val(r);
		}else if(i==1){
			$('input[name="keiyuPlaceKana2"]').val(r);
		}else if(i==2){
			$('input[name="keiyuPlaceKana3"]').val(r);
		}else if(i==3){
			$('input[name="keiyuPlaceKana4"]').val(r);
		}else if(i==4){
			$('input[name="keiyuPlaceKana5"]').val(r);
		}
		i++;
    	});

	$('input[name="priority"]').val(id);

	// 20200622 ADD
	$('#formSearchmAgain').attr('action', $('#formSearchmAgain').attr('action') + "#priceResult");

	//document.routesearchForm.submit();
	$('#formSearchmAgain').submit();
}

//----------------------------------------------
//1時間前後の検索実行
//----------------------------------------------
function fOneHourSubmit(index){
	
	setHiddenDate("js-departureICCal");

	$('input[name="keiyuPlaceKana"]').val("");
	$('input[name="keiyuPlaceKana2"]').val("");
	$('input[name="keiyuPlaceKana3"]').val("");
	// V12.0.0 ADD START
	$('input[name="keiyuPlaceKana4"]').val("");
	$('input[name="keiyuPlaceKana5"]').val("");
	// V12.0.0 ADD END
	
	var i = 0;
	// 経由地取得
	//$('.waypoint').children('input[type=text]').each(function() {
        $('.keiyuPlaceClass').each(function() {
        	var r = $(this).val();
		if(i==0){
			$('input[name="keiyuPlaceKana"]').val(r);
		}else if(i==1){
			$('input[name="keiyuPlaceKana2"]').val(r);
		}else if(i==2){
			$('input[name="keiyuPlaceKana3"]').val(r);
		}else if(i==3){
			$('input[name="keiyuPlaceKana4"]').val(r);
		}else if(i==4){
			$('input[name="keiyuPlaceKana5"]').val(r);
		}
		i++;
    	});

        var form = $('#js-departureICCal').closest('form')[0],
        year = form.searchYear.value,
        month = form.searchMonth.value - 1,
        day = form.searchDay.value,
        hour = form.searchHour.value,
        minute = form.searchMinute.value,
        time = moment(new Date(year, month, day, hour, minute));
	time.add('h', index);
 	form.searchYear.value = time.get('year');
    	form.searchMonth.value = time.get('month') + 1;
    	form.searchDay.value = time.get('date');
    	form.searchHour.value = time.get('hour');
    	form.searchMinute.value = time.get('minute');
	form.submit();

	return false;
}

//------------------------------------------------
// 割引リストポップアップで割引タイプ選択時の処理
//------------------------------------------------
function setSectionFee(sectionNumber, sectionType, sectionFee, sectionFeeNoUnit, sectionTypeNo){

	var reduce30 = 0;
	var reduce50 = 0;
	// ETCタイプが平日朝夕割りの場合は表示内容を変更
	if(sectionTypeNo == "501" || sectionTypeNo == "502" || sectionTypeNo == "503" || sectionTypeNo == "504"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduce" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduce" + sectionNumber).text();
		$("#section_type" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_fee" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC1" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduce" + sectionNumber).show();
		$("#currentReduce" + sectionNumber).hide();
	// ETCタイプが平日朝夕割り夜間1の場合
	}else if(sectionTypeNo == "505" || sectionTypeNo == "506" || sectionTypeNo == "507" || sectionTypeNo == "508"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduce_n1_" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduce_n1_" + sectionNumber).text();
		$("#section_type" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_fee" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC1" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduce" + sectionNumber).show();
		$("#currentReduce" + sectionNumber).hide();
	// ETCタイプが平日朝夕割り夜間2の場合
	}else if(sectionTypeNo == "509" || sectionTypeNo == "510" || sectionTypeNo == "511" || sectionTypeNo == "512"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduce_n2_" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduce_n2_" + sectionNumber).text();
		$("#section_type" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_fee" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC1" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduce" + sectionNumber).show();
		$("#currentReduce" + sectionNumber).hide();
	// V11.0.0 ADD START
	// ETCタイプが新深夜割引の場合
	}else if(sectionTypeNo == "50" || sectionTypeNo == "51" || sectionTypeNo == "54" || sectionTypeNo == "55"){
		var baseEtcNameForReduce = $("#baseEtcNameForLate" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForLate" + sectionNumber).text();
		$("#section_type" + sectionNumber).html(baseEtcNameForReduce);
		$("#section_feeETC1" + sectionNumber).html(baseEtcFeeForReduce);
	// V11.0.0 ADD END
	}else{
		$("#section_type" + sectionNumber).html(sectionType);
		//$("#section_fee" + sectionNumber).html(sectionFee);
		$("#section_feeETC1" + sectionNumber).html(sectionFeeNoUnit);

		$("#currentReduce" + sectionNumber).hide();
	}

	$("#feeListPopup" + sectionNumber).hide();

	// 料金区間の平日朝夕割の表示金額変更
	$("#selectedReduce30_" + sectionNumber).html(reduce30);
	$("#selectedReduce50_" + sectionNumber).html(reduce50);

	var isReduceSelected = "false";
	// 平日朝夕割引(30%還元)の合計金額変更
	var totalReduce30 = 0;
	for (var i=0; i<$(".routeReduce30_" + selectedKeiroNumber).length; i++) {
		var $nowItem = $(".routeReduce30_" + selectedKeiroNumber).eq(i);
		var numtxt = $nowItem.text();
		var num = new String(numtxt).replace(/,/g, "");
		if(num != "-"){ totalReduce30 += parseInt(num); }
		if($nowItem.is(":visible")){ isReduceSelected = "true"; }
	}
	var finishReduce30 = String(totalReduce30);
	if(finishReduce30 == "0"){
		$("#totalReduce30_" + selectedKeiroNumber).text("-");
	} else {
		$("#totalReduce30_" + selectedKeiroNumber).text(finishReduce30.replace(/^(-?\d+)(\d{3})/, "$1,$2"));
	}

	// 平日朝夕割引(50%還元)の合計金額変更
	var totalReduce50 = 0;
	for (var i=0; i<$(".routeReduce50_" + selectedKeiroNumber).length; i++) {
		var numtxt = $(".routeReduce50_" + selectedKeiroNumber).eq(i).text();
		var num = new String(numtxt).replace(/,/g, "");
		if(num != "-"){ totalReduce50 += parseInt(num); }
	}
	var finishReduce50 = String(totalReduce50);
	if(finishReduce50 == "0"){
		$("#totalReduce50_" + selectedKeiroNumber).text("-");
	} else {
		$("#totalReduce50_" + selectedKeiroNumber).text(finishReduce50.replace(/^(-?\d+)(\d{3})/, "$1,$2"));
	}

	if(isReduceSelected == "true"){
		//$("#totalDiscount" + selectedKeiroNumber).show();
		$("#totalDiscount" + selectedKeiroNumber).hide();
	} else {
		$("#totalDiscount" + selectedKeiroNumber).hide();
	}


	// ETCタイプが平日朝夕割りの場合は通常料金もしくはETC特別の料金をベースの料金とする。
	if(sectionTypeNo == "501" || sectionTypeNo == "502" || sectionTypeNo == "503" || sectionTypeNo == "504"){
		$("#selectedEtc1Fee" + sectionNumber).text($("#baseEtcFeeForReduce" + sectionNumber).text());
		$("#selectedEtc1No" + sectionNumber).text($("#baseEtcTypeForReduce" + sectionNumber).text());

		// V3.0.0 ADD START
		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduce" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
		// V3.0.0 ADD END
	// ETCタイプが平日朝夕割り夜間1の場合
	}else if(sectionTypeNo == "505" || sectionTypeNo == "506" || sectionTypeNo == "507" || sectionTypeNo == "508"){
		$("#selectedEtc1Fee" + sectionNumber).text($("#baseEtcFeeForReduce_n1_" + sectionNumber).text());
		$("#selectedEtc1No" + sectionNumber).text($("#baseEtcTypeForReduce_n1_" + sectionNumber).text());

		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduce_n1_" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// ETCタイプが平日朝夕割り夜間2の場合
	}else if(sectionTypeNo == "509" || sectionTypeNo == "510" || sectionTypeNo == "511" || sectionTypeNo == "512"){
		$("#selectedEtc1Fee" + sectionNumber).text($("#baseEtcFeeForReduce_n2_" + sectionNumber).text());
		$("#selectedEtc1No" + sectionNumber).text($("#baseEtcTypeForReduce_n2_" + sectionNumber).text());

		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduce_n2_" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// V11.0.0 ADD START
	// ETCタイプが新深夜割引の場合
	}else if(sectionTypeNo == "50" || sectionTypeNo == "51" || sectionTypeNo == "54" || sectionTypeNo == "55"){
		$("#selectedEtc1Fee" + sectionNumber).text($("#baseEtcFeeForLate" + sectionNumber).text());
		$("#selectedEtc1No" + sectionNumber).text($("#baseEtcTypeForLate" + sectionNumber).text());

		// 新深夜割引で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForLate" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// V11.0.0 ADD END
	} else {
		$("#selectedEtc1Fee" + sectionNumber).text(sectionFeeNoUnit);
		$("#selectedEtc1No" + sectionNumber).text(sectionTypeNo);
	}

	if($("#selectedEtcNo" + sectionNumber).text() == "1"){
		$("#section_type" + sectionNumber).parent("p").removeClass("cont_rootmap_price-etc");
	} else {
		$("#section_type" + sectionNumber).parent("p").removeClass("cont_rootmap_price-etc").addClass("cont_rootmap_price-etc");
	}

	// V3.0.0 ADD START
	if($("input[name='etc_radio_" + sectionNumber + "'].seamlessFeeArea_keiro" + selectedKeiroNumber).length || $("input[name='etc_radio_" + sectionNumber + "']").closest(".seamlessZeroArea_keiro" + selectedKeiroNumber).length){

	if(sectionTypeNo == "1"){
		// 通常料金が選択された場合はその経路のシームレス0円区間の割引エリアを表示し、通常料金を選択させる。
		$(".seamlessZeroAreaRadio_keiro" + selectedKeiroNumber).each( function() {
        		var $tmpElem = $(this).find("input[name^=etc_radio_]").first();// 1番目の要素が必ず通常料金ということにする。
			$tmpElem.prop('checked', true);
			var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
			$(this).closest(".js_etc_fee_area").find("[id^=section_feeETC1]").html(tmpFee);
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtc1Fee]").text(tmpFee.replace(/円/g,""));
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtc1No]").text("1");
		});
		$(".seamlessZeroArea_keiro" + selectedKeiroNumber + " dl.table-price").show();
		$("dt.seamlessZeroArea_keiro" + selectedKeiroNumber).show();
		$("dd.seamlessZeroArea_keiro" + selectedKeiroNumber).show();
		
	}else{
		// シームレス区間の0円が選択された場合は精算一本化エリアのシームレス料金を自動で選ぶ。
		if(sectionTypeNo == "40" && sectionFeeNoUnit == "0"){
			$(".seamlessFeeArea_keiro" + selectedKeiroNumber).each( function() {
        			var $tmpElem = $(this);
				$tmpElem.prop('checked', true);
				var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
				var $tmpElem2 = $tmpElem.closest(".js_etc_fee_area");
				$tmpElem2.find("[id^=section_feeETC1]").html(tmpFee);
				$tmpElem2.find("[id^=selectedEtc1Fee]").text(tmpFee.replace(/円/g,""));
				$tmpElem2.find("[id^=selectedEtc1No]").text("40");

				// 平日朝夕割エリアを非表示にする。
				//$tmpElem2.find("[id^=currentReduce]").hide();
			});
		}
		// 通常料金以外が選択された場合はその経路のシームレス0円区間の割引エリアを非表示にし、0円料金を選択させる。
		$(".seamlessZeroAreaRadio_keiro" + selectedKeiroNumber).each( function() {
        		var $tmpElem = $(this).find("input[name^=etc_radio_]:eq(1)");// 2番目の要素が必ず0円ということにする。
			$tmpElem.prop('checked', true);
			var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
			$(this).closest(".js_etc_fee_area").find("[id^=section_feeETC1]").html(tmpFee);
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtc1Fee]").text(tmpFee.replace(/円/g,""));
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtc1No]").text("40");
		});
		$(".seamlessZeroArea_keiro" + selectedKeiroNumber + " dl.table-price").hide();
		$("dt.seamlessZeroArea_keiro" + selectedKeiroNumber).hide();
		$("dd.seamlessZeroArea_keiro" + selectedKeiroNumber).hide();
	}
	}

	// シームレス料金表示時はメッセージを表示する。
	if($(".seamlessZeroArea_keiro" + selectedKeiroNumber).is(":hidden") || $(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).is(":hidden")){
		$(".seamlessMsgArea").show();
	}else{
		$(".seamlessMsgArea").hide();
	}

	// V3.0.0 ADD END


	// ETC割引合計用金設定
	var elems = document.getElementsByTagName("span");
	
	var name = "selectedEtcFeeClass" + selectedKeiroNumber;
  	
	var totalEtcFee = 0;

	for(var i = 0; i < elems.length; i++){
	    
		var classes = elems[i].className.split(" ");
		//var classes = elems[i].className;

		if(classes.indexOf(name) != -1){
			var num = new String($(elems[i]).text()).replace(/,/g, "");
			totalEtcFee += parseInt(num);
			//alert(num);
		}
	}


	var finishData = String(totalEtcFee);
	$("#fee_etc" + selectedKeiroNumber).text(finishData.replace(/^(-?\d+)(\d{3})/, "$1,$2"));


}

// V3.0.0 ADD START
//------------------------------------------------
// 割引リストポップアップで割引タイプ選択時の処理(ETC2.0)
//------------------------------------------------
function setSectionFeeETC2(sectionNumber, sectionType, sectionFee, sectionFeeNoUnit, sectionTypeNo){

	var reduce30 = 0;
	var reduce50 = 0;
	// ETCタイプが平日朝夕割りの場合は表示内容を変更
	if(sectionTypeNo == "501" || sectionTypeNo == "502" || sectionTypeNo == "503" || sectionTypeNo == "504"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduceETC2" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduceETC2" + sectionNumber).text();
		$("#section_typeETC2" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduceETC2" + sectionNumber).show();
		$("#currentReduceETC2" + sectionNumber).hide();
	// ETCタイプが平日朝夕割り夜間1の場合
	}else if(sectionTypeNo == "505" || sectionTypeNo == "506" || sectionTypeNo == "507" || sectionTypeNo == "508"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduceETC2_n1_" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduceETC2_n1_" + sectionNumber).text();
		$("#section_typeETC2" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduceETC2" + sectionNumber).show();
		$("#currentReduceETC2" + sectionNumber).hide();
	// ETCタイプが平日朝夕割り夜間2の場合
	}else if(sectionTypeNo == "509" || sectionTypeNo == "510" || sectionTypeNo == "511" || sectionTypeNo == "512"){
		var baseEtcNameForReduce = $("#baseEtcNameForReduceETC2_n2_" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForReduceETC2_n2_" + sectionNumber).text();
		$("#section_typeETC2" + sectionNumber).html(baseEtcNameForReduce);
		//$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce + "円");
		$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce);
		
		var reduceArray = sectionFeeNoUnit.split("|");
		reduce30 = (reduceArray[0]==0)? "-" : reduceArray[0];
		reduce50 = (reduceArray[1]==0)? "-" : reduceArray[1];

		//$("#currentReduceETC2" + sectionNumber).show();
		$("#currentReduceETC2" + sectionNumber).hide();
	// V11.0.0 ADD START
	// ETCタイプが新深夜割引の場合
	}else if(sectionTypeNo == "50" || sectionTypeNo == "51" || sectionTypeNo == "54" || sectionTypeNo == "55"){
		var baseEtcNameForReduce = $("#baseEtcNameForLateETC2" + sectionNumber).text();
		var baseEtcFeeForReduce = $("#baseEtcFeeForLateETC2" + sectionNumber).text();
		$("#section_typeETC2" + sectionNumber).html(baseEtcNameForReduce);
		$("#section_feeETC2" + sectionNumber).html(baseEtcFeeForReduce);
	// V11.0.0 ADD END
	}else{
		$("#section_typeETC2" + sectionNumber).html(sectionType);
		//$("#section_feeETC2" + sectionNumber).html(sectionFee);
		$("#section_feeETC2" + sectionNumber).html(sectionFeeNoUnit);

		$("#currentReduceETC2" + sectionNumber).hide();
	}

	$("#feeListPopupETC2" + sectionNumber).hide();

	// 料金区間の平日朝夕割の表示金額変更
	$("#selectedReduce30ETC2_" + sectionNumber).html(reduce30);
	$("#selectedReduce50ETC2_" + sectionNumber).html(reduce50);

	var isReduceSelected = "false";
	// 平日朝夕割引(30%還元)の合計金額変更
	var totalReduce30 = 0;
	for (var i=0; i<$(".routeReduce30ETC2_" + selectedKeiroNumber).length; i++) {
		var $nowItem = $(".routeReduce30ETC2_" + selectedKeiroNumber).eq(i);
		var numtxt = $nowItem.text();
		var num = new String(numtxt).replace(/,/g, "");
		if(num != "-"){ totalReduce30 += parseInt(num); }
		if($nowItem.is(":visible")){ isReduceSelected = "true"; }
	}
	var finishReduce30 = String(totalReduce30);
	if(finishReduce30 == "0"){
		$("#totalReduce30ETC2_" + selectedKeiroNumber).text("-");
	} else {
		$("#totalReduce30ETC2_" + selectedKeiroNumber).text(finishReduce30.replace(/^(-?\d+)(\d{3})/, "$1,$2"));
	}

	// 平日朝夕割引(50%還元)の合計金額変更
	var totalReduce50 = 0;
	for (var i=0; i<$(".routeReduce50ETC2_" + selectedKeiroNumber).length; i++) {
		var numtxt = $(".routeReduce50ETC2_" + selectedKeiroNumber).eq(i).text();
		var num = new String(numtxt).replace(/,/g, "");
		if(num != "-"){ totalReduce50 += parseInt(num); }
	}
	var finishReduce50 = String(totalReduce50);
	if(finishReduce50 == "0"){
		$("#totalReduce50ETC2_" + selectedKeiroNumber).text("-");
	} else {
		$("#totalReduce50ETC2_" + selectedKeiroNumber).text(finishReduce50.replace(/^(-?\d+)(\d{3})/, "$1,$2"));
	}

	if(isReduceSelected == "true"){
		//$("#totalDiscountETC2" + selectedKeiroNumber).show();
		$("#totalDiscountETC2" + selectedKeiroNumber).hide();
	} else {
		$("#totalDiscountETC2" + selectedKeiroNumber).hide();
	}


	// ETCタイプが平日朝夕割りの場合は通常料金もしくはETC特別の料金をベースの料金とする。
	if(sectionTypeNo == "501" || sectionTypeNo == "502" || sectionTypeNo == "503" || sectionTypeNo == "504"){
		$("#selectedEtcFeeETC2" + sectionNumber).text($("#baseEtcFeeForReduceETC2" + sectionNumber).text());
		$("#selectedEtcNoETC2" + sectionNumber).text($("#baseEtcTypeForReduceETC2" + sectionNumber).text());

		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduceETC2" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// ETCタイプが平日朝夕割り夜間1の場合
	}else if(sectionTypeNo == "505" || sectionTypeNo == "506" || sectionTypeNo == "507" || sectionTypeNo == "508"){
		$("#selectedEtcFeeETC2" + sectionNumber).text($("#baseEtcFeeForReduceETC2_n1_" + sectionNumber).text());
		$("#selectedEtcNoETC2" + sectionNumber).text($("#baseEtcTypeForReduceETC2_n1_" + sectionNumber).text());

		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduceETC2_n1_" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// ETCタイプが平日朝夕割り夜間2の場合
	}else if(sectionTypeNo == "509" || sectionTypeNo == "510" || sectionTypeNo == "511" || sectionTypeNo == "512"){
		$("#selectedEtcFeeETC2" + sectionNumber).text($("#baseEtcFeeForReduceETC2_n2_" + sectionNumber).text());
		$("#selectedEtcNoETC2" + sectionNumber).text($("#baseEtcTypeForReduceETC2_n2_" + sectionNumber).text());

		// 平日朝夕割で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForReduceETC2_n2_" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// V11.0.0 ADD START
	// ETCタイプが新深夜割引の場合
	}else if(sectionTypeNo == "50" || sectionTypeNo == "51" || sectionTypeNo == "54" || sectionTypeNo == "55"){
		$("#selectedEtcFeeETC2" + sectionNumber).text($("#baseEtcFeeForLateETC2" + sectionNumber).text());
		$("#selectedEtcNoETC2" + sectionNumber).text($("#baseEtcTypeForLateETC2" + sectionNumber).text());

		// 新深夜割引で通常料金だった場合でシームレスだった場合の対応
		if($("#baseEtcTypeForLateETC2" + sectionNumber).text() == "1"){
			sectionTypeNo = 1;
		}
	// V11.0.0 ADD END
	} else {
		$("#selectedEtcFeeETC2" + sectionNumber).text(sectionFeeNoUnit);
		$("#selectedEtcNoETC2" + sectionNumber).text(sectionTypeNo);
	}

	if($("#selectedEtcNoETC2" + sectionNumber).text() == "1"){
		$("#section_typeETC2" + sectionNumber).parent("p").removeClass("cont_rootmap_price-etc");
	} else {
		$("#section_typeETC2" + sectionNumber).parent("p").removeClass("cont_rootmap_price-etc").addClass("cont_rootmap_price-etc");
	}

	if($("input[name='etc_radio_etc2_" + sectionNumber + "'].seamlessFeeAreaETC2_keiro" + selectedKeiroNumber).length || $("input[name='etc_radio_etc2_" + sectionNumber + "']").closest(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).length){
	//if($("#feeListPopupETC2" + sectionNumber).find(".seamlessFeeAreaETC2_keiro" + selectedKeiroNumber).length || $("#feeListPopupETC2" + sectionNumber).closest(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).length){
	if(sectionTypeNo == "1"){
		// 通常料金が選択された場合はその経路のシームレス0円区間の割引エリアを表示し、通常料金を選択させる。
		$(".seamlessZeroAreaRadioETC2_keiro" + selectedKeiroNumber).each( function() {
			var $tmpElem = $(this).find("input[name^=etc_radio_etc2_]").first();// 1番目の要素が必ず通常料金ということにする。
			$tmpElem.prop('checked', true);
			var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
			$(this).closest(".js_etc_fee_area").find("[id^=section_feeETC2]").html(tmpFee);
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtcFeeETC2]").text(tmpFee.replace(/円/g,""));
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtcNoETC2]").text("1");
		});
		$(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber + " dl.table-price").show();
		$("dt.seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).show();
		$("dd.seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).show();
		
	}else{
		// シームレス区間の0円が選択された場合は精算一本化エリアのシームレス料金を自動で選ぶ。
		if(sectionTypeNo == "40" && sectionFeeNoUnit == "0"){
			$(".seamlessFeeAreaETC2_keiro" + selectedKeiroNumber).each( function() {
				var $tmpElem = $(this);
				$tmpElem.prop('checked', true);
				var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
				var $tmpElem2 = $tmpElem.closest(".js_etc_fee_area");
				$tmpElem2.find("[id^=section_typeETC2]").html(tmpFee);
				$tmpElem2.find("[id^=selectedEtcFeeETC2]").text(tmpFee.replace(/円/g,""));
				$tmpElem2.find("[id^=selectedEtcNoETC2]").text("40");

				// 平日朝夕割エリアを非表示にする。
				//$tmpElem2.find("[id^=currentReduceETC2]").hide();
			});
		}
		// 通常料金以外が選択された場合はその経路のシームレス0円区間の割引エリアを非表示にし、0円料金を選択させる。
		$(".seamlessZeroAreaRadioETC2_keiro" + selectedKeiroNumber).each( function() {
        		var $tmpElem = $(this).find(".cont_rootmap_win-item:eq(1)");// 2番目の要素が必ず0円ということにする。
			$tmpElem.addClass('current').siblings().removeClass('current');
			var tmpFee = $tmpElem.find("dd").text();
			$(this).find("[id^=section_feeETC2]").html(tmpFee);
			var tmpTypeName = $tmpElem.find("dt").text();
			$(this).find("[id^=section_typeETC2]").html(tmpTypeName);
			$(this).find("[id^=section_typeETC2]").parent("p").removeClass("cont_rootmap_price-etc").addClass("cont_rootmap_price-etc");
			$(this).find("[id^=selectedEtcFeeETC2]").text(tmpFee.replace(/円/g,""));
			$(this).find("[id^=selectedEtcNoETC2]").text("40");

			var $tmpElem = $(this).find("input[name^=etc_radio_etc2_]:eq(1)");// 2番目の要素が必ず0円ということにする。
			$tmpElem.prop('checked', true);
			var tmpFee = $tmpElem.closest("dt").next("dd").find("em").text();
			$(this).closest(".js_etc_fee_area").find("[id^=section_typeETC2]").html(tmpFee);
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtcFeeETC2]").text(tmpFee.replace(/円/g,""));
			$(this).closest(".js_etc_fee_area").find("[id^=selectedEtcNoETC2]").text("40");

		});
		$(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber + " dl.table-price").hide();
		$("dt.seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).hide();
		$("dd.seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).hide();
	}
	}

	// シームレス料金表示時はメッセージを表示する。
	if($(".seamlessZeroArea_keiro" + selectedKeiroNumber).is(":hidden") || $(".seamlessZeroAreaETC2_keiro" + selectedKeiroNumber).is(":hidden")){
		$(".seamlessMsgArea").show();
	}else{
		$(".seamlessMsgArea").hide();
	}

	// ETC割引合計用金設定
	var elems = document.getElementsByTagName("span");
	
	var name = "selectedEtcFeeClassETC2" + selectedKeiroNumber;
  	
	var totalEtcFee = 0;

	for(var i = 0; i < elems.length; i++){
	    
		var classes = elems[i].className.split(" ");
		//var classes = elems[i].className;

		if(classes.indexOf(name) != -1){
			var num = new String($(elems[i]).text()).replace(/,/g, "");
			totalEtcFee += parseInt(num);
			//alert(num);
		}
	}


	var finishData = String(totalEtcFee);
	$("#fee_etc2" + selectedKeiroNumber).text(finishData.replace(/^(-?\d+)(\d{3})/, "$1,$2"));


}
// V3.0.0 ADD END


//----------------------------------------------
// 追従 再検索ボタン押下時の処理
//----------------------------------------------
function fSubmit_follow() {
	
	isFollowBtn = true;
	$('#formSearch_follow').attr('action', constDpSearchQuick_List[drLangCode]);
	setHiddenDate("js-departureICCal1");

	// 追従検索に表示されてない検索条件をデフォルトに設定する 2020/3/26
	//$('#formSearch_follow input[name="keiyuPlaceKana"]').val("");
	//$('#formSearch_follow input[name="keiyuPlaceKana2"]').val("");
	//$('#formSearch_follow input[name="keiyuPlaceKana3"]').val("");
	//$('#formSearch_follow input[name="carType"]').val("1");
	//$('#formSearch_follow input[name="priority"]').val("2");
	//$('#formSearch_follow input[name="roadType1"]').val("off");
	//$('#formSearch_follow input[name="roadType2"]').val("off");
	//$('#formSearch_follow input[name="roadType"]').val("15");
}

//----------------------------------------------
// 検索トップへボタン押下時の処理
//----------------------------------------------
function goSearchTop_follow() {
	
	isFollowBtn = true;
	setHiddenDate("js-departureICCal1");
	$('#formSearch_follow').attr('action', constDpSearchTop_List[drLangCode]);
	$('#formSearch_follow').submit();
	//return false;
}

//----------------------------------------------
// 渋滞・規制ページ表示
//----------------------------------------------
function goTraffic(lat,lon) {

	document.formTraffic.k.value = "";
	document.formTraffic.icfrom.value = "";
	document.formTraffic.icvia.value = "";
	
	document.formTraffic.icvia2.value = "";
	document.formTraffic.icvia3.value = "";
	// REV 12.0.0 ADD START
	document.formTraffic.icvia4.value = "";
	document.formTraffic.icvia5.value = "";
	// REV 12.0.0 ADD END
	
	document.formTraffic.icto.value = "";
	document.formTraffic.ctype.value = "";
	document.formTraffic.priority.value = "";

	document.formTraffic.time.value = "";

	document.formTraffic.rno.value = "";
	document.formTraffic.maxresult.value = "";

	document.formTraffic.lat.value = lat;
	document.formTraffic.lon.value = lon;

	document.formTraffic.roadtype.value = "";

	document.formTraffic.target = "_Traffic";
	document.formTraffic.action = sDriveTrafficURL + "/map.html";
	document.formTraffic.submit();
}

//----------------------------------------------
// SAPAブログページ表示
//----------------------------------------------
function goSapaBlog(url) {
	//var param = "?icf="+encodeURI(document.formSearchmAgain.startPlace.value)+"&ict="+encodeURI(document.formSearchmAgain.arrivePlace.value)+"&ick="+encodeURI(document.formSearchmAgain.keiyuPlace.value);
	var icf = $('#formSearchmAgain input[name="startPlaceKana"]').val();
	var ict = $('#formSearchmAgain input[name="arrivePlaceKana"]').val();
	var ick = $('#formSearchmAgain input[name="keiyuPlaceKana"]').val();
	var param = "?icf="+encodeURI(icf)+"&ict="+encodeURI(ict)+"&ick="+encodeURI(ick);
	window.open(url+param);
}
//----------------------------------------------
// ルートマップ初期化処理
//----------------------------------------------
function init(){
	//各種アイコンをセット
	// 料金検索エンジン改良対応 MOD START
	//MapSetStartNIC(initStartIc);
	//MapSetArrivalNIC(initEndIc);
	//var route_st_ic = $('.cont_rootmap_head-start').eq(selectedKeiroNumber - 1).attr('data-iccode');
	//var route_ar_ic = $('.cont_rootmap_head-arrival').eq(selectedKeiroNumber - 1).attr('data-iccode');
	var route_st_ic = $('dl.start').eq(selectedKeiroNumber - 1).attr('data-iccode');
	var route_ar_ic = $('dl.goal').eq(selectedKeiroNumber - 1).attr('data-iccode');
	MapSetStartNIC_ifm(route_st_ic);
	MapSetArrivalNIC_ifm(route_ar_ic);
	// 料金検索エンジン改良対応 MOD END

	if(initThroughIc != ""){
		MapSetThroughNIC_ifm(initThroughIc);
	}
	if(initThroughIc2 != ""){
		MapSetThroughNIC_ifm(initThroughIc2);
	}
	if(initThroughIc3 != ""){
		MapSetThroughNIC_ifm(initThroughIc3);
	}
	// V12.0.0 ADD START
	if(initThroughIc4 != ""){
		MapSetThroughNIC_ifm(initThroughIc4);
	}
	if(initThroughIc5 != ""){
		MapSetThroughNIC_ifm(initThroughIc5);
	}
	// V12.0.0 ADD END

	MapChangeLayer_ifm(0);
	
	//カレントレイヤの地図配置（出発ICでセンタリング）
	MapSetPositionbyIC_ifm(initStartIc);
}
function MapSetStartNIC_ifm(ic){
	document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
	document.getElementById("iframeRouteMap").contentWindow.MapSetStartNIC_ifm(ic);
}
function MapSetArrivalNIC_ifm(ic){
	document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
	document.getElementById("iframeRouteMap").contentWindow.MapSetArrivalNIC_ifm(ic);
}
function MapSetThroughNIC_ifm(ic){
	document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
	document.getElementById("iframeRouteMap").contentWindow.MapSetThroughNIC_ifm(ic);
}
function MapChangeLayer_ifm(id){
	document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
	document.getElementById("iframeRouteMap").contentWindow.MapChangeLayer_ifm(id);
}
function MapSetPositionbyIC_ifm(ic){
	document.getElementById("iframeRouteMap").contentWindow.selectedKeiroNumber = selectedKeiroNumber;
	document.getElementById("iframeRouteMap").contentWindow.MapSetPositionbyIC_ifm(ic);
}

//----------------------------------------------
// ルートマップ内SAPAクリック時処理
//----------------------------------------------
function onClickSAPA(id, name, kana){
	if (id.length==7){
		showSAPABalloon(id, name, kana);
	}
}
//----------------------------------------------
// ルートマップ内SAPAバルーン表示処理
//----------------------------------------------
function showSAPABalloon(ic, name, kana){
	tmpIcCode = ic;
	var innerHTML = "";	
	
	if(ic == "1830071"){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_UP_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + constUdClass_DOWN_List[drLangCode];
	}else if(ic.substring(0,4) == "1461"){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_WEST_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_EAST_List[drLangCode] + "</a>";
	}else if(ic.substring(0,4) == "214K"){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_IN_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_OUT_List[drLangCode] + "</a>";
	}else if(
			ic == "1050033" || ic == "5004016" || ic == "1101186" || ic == "1830031" || ic == "1830049" ||
			ic == "1612006" || ic == "1202006" || ic == "1030036" || ic == "1311076" ||
			ic == "1312002" || ic == "5009021" || ic == "5012006" || ic == "1810003"
	){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_UP_List[drLangCode] + "</a>";
	}else if(
			ic == "5004031" || ic == "1011019" || ic == "1830036" || ic == "1830056" ||
			ic == "1073031" || ic == "1612011" || ic == "1202011" || ic == "1030041" ||
			ic == "1311071" || ic == "1312004" || ic == "5009016" || ic == "5009036" ||
			ic == "5012011" || ic == "213A016"
	){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_DOWN_List[drLangCode] + "</a>";
	}else if(ic == "1040041"){
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_UP_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='/pasar/hanyu/' target='sapa'>" + constUdClass_DOWN_List[drLangCode] + "</a>";
	}else if(ic == "1800011"){
		innerHTML = innerHTML + "<a href='/pasar/miyoshi/' target='sapa'>" + constUdClass_UP_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_DOWN_List[drLangCode] + "</a>";
	}else if(ic == "213A041"){
		window.open('/pasar/makuhari/','PASAR幕張','');
		return;
	}else if(ic == "1400016"){
		innerHTML = innerHTML + "<a href='/pasar/moriya/' target='sapa'>" + constUdClass_UP_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_DOWN_List[drLangCode] + "</a>";
	}else{
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"1\")'>" + constUdClass_UP_List[drLangCode] + "</a><br>";
		innerHTML = innerHTML + "<a href='javascript:actionSAPA(\"2\")'>" + constUdClass_DOWN_List[drLangCode] + "</a>";
	}
	MapShowBalloon(kana, name, innerHTML);
}
//----------------------------------------------
// ルートマップ内SAPAページ表示処理
//----------------------------------------------
function actionSAPA(updown){
/*
	var param = "?icf="+encodeURI(document.formSearchmAgain.startPlace.value)+"&ict="+encodeURI(document.formSearchmAgain.arrivePlace.value)+"&ick="+encodeURI(document.formSearchmAgain.keiyuPlace.value);
	document.formSearchmAgain.action="/sapa/"+tmpIcCode.substring(0,4)+"/"+tmpIcCode+"/"+updown+"/" + param;
	document.formSearchmAgain.target="sapa";
	document.formSearchmAgain.submit();
*/

	var icf = $('#formSearchmAgain input[name="startPlaceKana"]').val();
	var ict = $('#formSearchmAgain input[name="arrivePlaceKana"]').val();
	var ick = $('#formSearchmAgain input[name="keiyuPlaceKana"]').val();

	var param = "?icf="+encodeURI(icf)+"&ict="+encodeURI(ict)+"&ick="+encodeURI(ick);

	var url = "/sapa/"+tmpIcCode.substring(0,4)+"/"+tmpIcCode+"/"+updown+"/" + param;
	window.open(url);


}
//----------------------------------------------
// ルートマップ内レイヤー切替時イベント
//----------------------------------------------
/*
function onChangeLayer(layer){

	// 検索条件画面ならリターン
	if(selectedKeiroNumber == 99){
		return;
	}

	var tr = "";
//alert($.fn.jquery);
	switch(selectedKeiroNumber){
		case 1:
			switch(layer){
				case 0:
					if(traceData1["japan"]!=undefined){
						var idx = traceData1["japan"].length-1;
						if(traceData1["japan"][idx]!=undefined){
							tr = traceData1["japan"][idx];
						}
					}
					break;
				case 1:
					if(traceData1["tokyo"]!=undefined){
						var idx = traceData1["tokyo"].length-1;
						if(traceData1["tokyo"][idx]!=undefined){
							tr = traceData1["tokyo"][idx];
						}
					}
					break;
				case 2:
					if(traceData1["osaka"]!=undefined){
						var idx = traceData1["osaka"].length-1;
						if(traceData1["osaka"][idx]!=undefined){
							tr = traceData1["osaka"][idx];
						}
					}
					break;
				case 3:
					if(traceData1["nagoya"]!=undefined){
						var idx = traceData1["nagoya"].length-1;
						if(traceData1["nagoya"][idx]!=undefined){
							tr = traceData1["nagoya"][idx];
						}
					}
					break;
				case 4:
					if(traceData1["kitakyusyu"]!=undefined){
						var idx = traceData1["kitakyusyu"].length-1;
						if(traceData1["kitakyusyu"][idx]!=undefined){
							tr = traceData1["kitakyusyu"][idx];
						}
					}
					break;
				default:
					break;
			}
			break;
		case 2:
			switch(layer){
				case 0:
					if(traceData2["japan"]!=undefined){
						var idx = traceData2["japan"].length-1;
						if(traceData2["japan"][idx]!=undefined){
							tr = traceData2["japan"][idx];
						}
					}
					break;
				case 1:
					if(traceData2["tokyo"]!=undefined){
						var idx = traceData2["tokyo"].length-1;
						if(traceData2["tokyo"][idx]!=undefined){
							tr = traceData2["tokyo"][idx];
						}
					}
					break;
				case 2:
					if(traceData2["osaka"]!=undefined){
						var idx = traceData2["osaka"].length-1;
						if(traceData2["osaka"][idx]!=undefined){
							tr = traceData2["osaka"][idx];
						}
					}
					break;
				case 3:
					if(traceData2["nagoya"]!=undefined){
						var idx = traceData2["nagoya"].length-1;
						if(traceData2["nagoya"][idx]!=undefined){
							tr = traceData2["nagoya"][idx];
						}
					}
					break;
				case 4:
					if(traceData2["kitakyusyu"]!=undefined){
						var idx = traceData2["kitakyusyu"].length-1;
						if(traceData2["kitakyusyu"][idx]!=undefined){
							tr = traceData2["kitakyusyu"][idx];
						}
					}
					break;
				default:
					break;
			}
			break;
		case 3:
			switch(layer){
				case 0:
					if(traceData3["japan"]!=undefined){
						var idx = traceData3["japan"].length-1;
						if(traceData3["japan"][idx]!=undefined){
							tr = traceData3["japan"][idx];
						}
					}
					break;
				case 1:
					if(traceData3["tokyo"]!=undefined){
						var idx = traceData3["tokyo"].length-1;
						if(traceData3["tokyo"][idx]!=undefined){
							tr = traceData3["tokyo"][idx];
						}
					}
					break;
				case 2:
					if(traceData3["osaka"]!=undefined){
						var idx = traceData3["osaka"].length-1;
						if(traceData3["osaka"][idx]!=undefined){
							tr = traceData3["osaka"][idx];
						}
					}
					break;
				case 3:
					if(traceData3["nagoya"]!=undefined){
						var idx = traceData3["nagoya"].length-1;
						if(traceData3["nagoya"][idx]!=undefined){
							tr = traceData3["nagoya"][idx];
						}
					}
					break;
				case 4:
					if(traceData3["kitakyusyu"]!=undefined){
						var idx = traceData3["kitakyusyu"].length-1;
						if(traceData3["kitakyusyu"][idx]!=undefined){
							tr = traceData3["kitakyusyu"][idx];
						}
					}
					break;
				default:
					break;
			}
			break;
		case 4:
			switch(layer){
				case 0:
					if(traceData4["japan"]!=undefined){
						var idx = traceData4["japan"].length-1;
						if(traceData4["japan"][idx]!=undefined){
							tr = traceData4["japan"][idx];
						}
					}
					break;
				case 1:
					if(traceData4["tokyo"]!=undefined){
						var idx = traceData4["tokyo"].length-1;
						if(traceData4["tokyo"][idx]!=undefined){
							tr = traceData4["tokyo"][idx];
						}
					}
					break;
				case 2:
					if(traceData4["osaka"]!=undefined){
						var idx = traceData4["osaka"].length-1;
						if(traceData4["osaka"][idx]!=undefined){
							tr = traceData4["osaka"][idx];
						}
					}
					break;
				case 3:
					if(traceData4["nagoya"]!=undefined){
						var idx = traceData4["nagoya"].length-1;
						if(traceData4["nagoya"][idx]!=undefined){
							tr = traceData4["nagoya"][idx];
						}
					}
					break;
				case 4:
					if(traceData4["kitakyusyu"]!=undefined){
						var idx = traceData4["kitakyusyu"].length-1;
						if(traceData4["kitakyusyu"][idx]!=undefined){
							tr = traceData4["kitakyusyu"][idx];
						}
					}
					break;
				default:
					break;
			}
			break;
		case 5:
			switch(layer){
				case 0:
					if(traceData5["japan"]!=undefined){
						var idx = traceData5["japan"].length-1;
						if(traceData5["japan"][idx]!=undefined){
							tr = traceData5["japan"][idx];
						}
					}
					break;
				case 1:
					if(traceData5["tokyo"]!=undefined){
						var idx = traceData5["tokyo"].length-1;
						if(traceData5["tokyo"][idx]!=undefined){
							tr = traceData5["tokyo"][idx];
						}
					}
					break;
				case 2:
					if(traceData5["osaka"]!=undefined){
						var idx = traceData5["osaka"].length-1;
						if(traceData5["osaka"][idx]!=undefined){
							tr = traceData5["osaka"][idx];
						}
					}
					break;
				case 3:
					if(traceData5["nagoya"]!=undefined){
						var idx = traceData5["nagoya"].length-1;
						if(traceData5["nagoya"][idx]!=undefined){
							tr = traceData5["nagoya"][idx];
						}
					}
					break;
				case 4:
					if(traceData5["kitakyusyu"]!=undefined){
						var idx = traceData5["kitakyusyu"].length-1;
						if(traceData5["kitakyusyu"][idx]!=undefined){
							tr = traceData5["kitakyusyu"][idx];
						}
					}
					break;
				default:
					break;
			}
			break;
		
		default:
			break;
	}
	//lineタグ情報を用いてラインを表示する。
	//ラインが存在しないレイヤに切り替えた場合(tr="")はラインを（積極的に）消す。
	MapShowResultLine(tr);
}
*/

// マイルート登録
function goMyRoute(num) {
	
	document.formTraffic.k.value = "";

	//document.formTraffic.icfrom.value = document.formSearchmAgain.startPlaceCode.value;
	//document.formTraffic.icvia.value = document.formSearchmAgain.keiyuPlaceCode.value;
	//document.formTraffic.icvia2.value = document.formSearchmAgain.keiyuPlaceCode2.value;
	//document.formTraffic.icvia3.value = document.formSearchmAgain.keiyuPlaceCode3.value;
	//document.formTraffic.icto.value = document.formSearchmAgain.arrivePlaceCode.value;
	document.formTraffic.icfrom.value = initStartIc;
	document.formTraffic.icvia.value = initThroughIc;
	document.formTraffic.icvia2.value = initThroughIc2;
	document.formTraffic.icvia3.value = initThroughIc3;
	// REV12.0.0 ADD START
	document.formTraffic.icvia4.value = initThroughIc4;
	document.formTraffic.icvia5.value = initThroughIc5;
	// REV12.0.0 ADD END
	document.formTraffic.icto.value = initEndIc;


	//document.formTraffic.ctype.value = parseInt(document.formSearchmAgain.carType.value) + 1;
	document.formTraffic.ctype.value = parseInt($('#formSearchmAgain select[name="carType"]').val()) + 1;

	//document.formTraffic.priority.value = $('input[name="priority"]:checked').val();
	document.formTraffic.priority.value = $('#formSearchmAgain input[name="priority"]:checked').val();

	//var yyyy = document.formSearchmAgain.searchYear.value;
	var yyyy = $('#formSearchmAgain input[name="searchYear"]').val();
	//var mm = document.formSearchmAgain.searchMonth.value;
	var mm = $('#formSearchmAgain input[name="searchMonth"]').val();
	if(mm.length == 1) mm = "0" + mm;
	//var dd = document.formSearchmAgain.searchDay.value;
	var dd = $('#formSearchmAgain input[name="searchDay"]').val();
	if(dd.length == 1) dd = "0" + dd;
	//var hh = document.formSearchmAgain.searchHour.value;
	var hh = $('#formSearchmAgain select[name="searchHour"]').val();
	if(hh.length == 1) hh = "0" + hh;
	//var mi = document.formSearchmAgain.searchMinute.value;
	var mi = $('#formSearchmAgain select[name="searchMinute"]').val();
	if(mi.length == 1) mi = "0" + mi;
	var time = yyyy + mm + dd + hh + mi;
	document.formTraffic.time.value = time;
	//document.formTraffic.kind.value = document.formSearchmAgain.kind.value;
	//document.formTraffic.kind.value = $('#formSearchmAgain input[name="kind"]').val();
	document.formTraffic.kind.value = $('#formSearchmAgain input[name="kind"]:checked').val();

	document.formTraffic.rno.value = num;
	//document.formTraffic.maxresult.value = $('input[name="routeNum"]:checked').val();
	document.formTraffic.maxresult.value = 3;

	document.formTraffic.lat.value = "";
	document.formTraffic.lon.value = "";

	//document.formTraffic.roadtype.value = document.formSearchmAgain.roadType.value;
	//document.formTraffic.roadtype.value = $('#formSearchmAgain input[name="roadType"]').val();
	document.formTraffic.roadtype.value = $('#formSearch_follow input[name="roadType"]').val();


	document.formTraffic.target = "";

	if (winResizeWidth >= 768) {
		// PC
		document.formTraffic.action = sDriveTrafficURL + "/myroute_exe.html";
	}else{
		// SP
		document.formTraffic.action = sDriveTrafficURL + "/routeent_exe.html";
	}

	document.formTraffic.submit();
}

// 経路内SAPA検索
function goSAPASearch(num) {

	var icf = $('#formSearchmAgain input[name="startPlaceKana"]').val();
	var ict = $('#formSearchmAgain input[name="arrivePlaceKana"]').val();

	document.formSearchSAPA.startIcName.value = icf;
	document.formSearchSAPA.arriveIcName.value = ict;
	document.formSearchSAPA.keiroCodeCSV.value = eval("keiroCodeCSV" + (num-1));
	//document.formSearchSAPA.target = "sapa_search";
	document.formSearchSAPA.target = "";
	document.formSearchSAPA.action = "/dp/SAPAService";
	document.formSearchSAPA.submit();
}

// 印刷画面へ
function goPrint(num) {

	var p_etcFee = "";
	var p_etcFee2 = "";
	var p_selectetc = "";
	var p_selectetc2 = "";
	var p_finishReduce30 = "";
	var p_finishReduce50 = "";
	var p_finishReduce30ETC2 = "";
	var p_finishReduce50ETC2 = "";
	var p_selectReduce30 = "";
	var p_selectReduce50 = "";
	var p_selectReduce30ETC2 = "";
	var p_selectReduce50ETC2 = "";
	// V11.0.0 ADD START
	var p_selectLateReb = "";
	var p_selectLateDis = "";
	var p_selectLateRebE22 = "";
	var p_selectLateDisE22 = "";
	var p_selectLateRebETC2 = "";
	var p_selectLateDisETC2 = "";
	var p_selectLateRebE22ETC2 = "";
	var p_selectLateDisE22ETC2 = "";
	// V11.0.0 ADD END
	// V12.0.0 ADD START
	var p_selectReduce30Dis = "";
	var p_selectReduce50Dis = "";
	var p_selectReduce30DisETC2 = "";
	var p_selectReduce50DisETC2 = "";
	// V12.0.0 ADD END



	// ETC合計料金設定
	p_etcFee = $("#fee_etc" + selectedKeiroNumber).text();
	// ETC2.0合計料金設定
	p_etcFee2 = $("#fee_etc2" + selectedKeiroNumber).text();

	// 選択されたETC割引タイプ取得
	var elems = document.getElementsByTagName("span");
	
	var name = "selectedEtcNoClass" + num;
	var name2 = "selectedEtcNoClassETC2" + num;

	var s_etc_param = "";
  	var s_etc_param2 = "";
  	
	var speNo = 1;
	var speNo2 = 1;
	for(var i = 0; i < elems.length; i++){
	    
		var classes = elems[i].className.split(" ");
	
		if(classes.indexOf(name) != -1){
			
			//s_etc_param += "&etcSpecial[" + speNo + "]=";

			if(speNo != 1){
				s_etc_param += "_";
			}

			s_etc_param += $(elems[i]).text();
			
			speNo++;
		}

		// ETC2.0
		if(classes.indexOf(name2) != -1){
			
			if(speNo2 != 1){
				s_etc_param2 += "_";
			}

			s_etc_param2 += $(elems[i]).text();
			
			speNo2++;
		}
	}

	p_selectetc = s_etc_param;
	p_selectetc2 = s_etc_param2;

	// 朝夕割引合計金額設定
	if($("#totalReduce30_" + selectedKeiroNumber).is(':visible')){
		p_finishReduce30 = $("#totalReduce30_" + selectedKeiroNumber).text();
	} else {
		p_finishReduce30 = "empty";
	}
	if($("#totalReduce50_" + selectedKeiroNumber).is(':visible')){
		p_finishReduce50 = $("#totalReduce50_" + selectedKeiroNumber).text();
	} else {
		p_finishReduce50 = "empty";
	}

	// 朝夕割引合計金額設定(ETC2.0)
	if($("#totalReduce30ETC2_" + selectedKeiroNumber).is(':visible')){
		p_finishReduce30ETC2 = $("#totalReduce30ETC2_" + selectedKeiroNumber).text();
	} else {
		p_finishReduce30ETC2 = "empty";
	}
	if($("#totalReduce50ETC2_" + selectedKeiroNumber).is(':visible')){
		p_finishReduce50ETC2 = $("#totalReduce50ETC2_" + selectedKeiroNumber).text();
	} else {
		p_finishReduce50ETC2 = "empty";
	}
	
	
	// 選択された朝夕割引30%還元値設定
	var selectReduce30 = "";
	//$(".print_routeReduce30_" + selectedKeiroNumber).each( function() {
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		//var $tmpElem = $(this).closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce30 != "") {
			selectReduce30 += "_";
		}
		selectReduce30 += numtxt;
	});
	p_selectReduce30 = selectReduce30;

	// 選択された朝夕割引50%還元値設定
	var selectReduce50 = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce50 != "") {
			selectReduce50 += "_";
		}
		selectReduce50 += numtxt;
	});
	p_selectReduce50 = selectReduce50;

	// 選択された朝夕割引30%還元値設定(ETC2.0)
	var selectReduce30ETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce30ETC2 != "") {
			selectReduce30ETC2 += "_";
		}
		selectReduce30ETC2 += numtxt;
	});
	p_selectReduce30ETC2 = selectReduce30ETC2;

	// 選択された朝夕割引50%還元値設定(ETC2.0)
	var selectReduce50ETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce50ETC2 != "") {
			selectReduce50ETC2 += "_";
		}
		selectReduce50ETC2 += numtxt;
	});
	p_selectReduce50ETC2 = selectReduce50ETC2;

	// V12.0.0 ADD START
	// 選択された朝夕割引30%割引額設定
	var selectReduce30Dis = "";
	//$(".print_routeReduce30_" + selectedKeiroNumber).each( function() {
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		//var $tmpElem = $(this).closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30Dis_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30Dis_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30Dis_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce30Dis != "") {
			selectReduce30Dis += "_";
		}
		selectReduce30Dis += numtxt;
	});
	p_selectReduce30Dis = selectReduce30Dis;

	// 選択された朝夕割引50%割引額設定
	var selectReduce50Dis = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50Dis_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50Dis_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50Dis_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce50Dis != "") {
			selectReduce50Dis += "_";
		}
		selectReduce50Dis += numtxt;
	});
	p_selectReduce50Dis = selectReduce50Dis;

	// 選択された朝夕割引30%割引額設定(ETC2.0)
	var selectReduce30DisETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2Dis_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2Dis_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce30ETC2Dis_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce30DisETC2 != "") {
			selectReduce30DisETC2 += "_";
		}
		selectReduce30DisETC2 += numtxt;
	});
	p_selectReduce30DisETC2 = selectReduce30DisETC2;

	// 選択された朝夕割引50%割引額設定(ETC2.0)
	var selectReduce50DisETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2Dis_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2Dis_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeReduce50ETC2Dis_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectReduce50DisETC2 != "") {
			selectReduce50DisETC2 += "_";
		}
		selectReduce50DisETC2 += numtxt;
	});
	p_selectReduce50DisETC2 = selectReduce50DisETC2;
	// V12.0.0 ADD END

	// V11.0.0 ADD START
	// 新深夜割引設定
	var selectLateReb = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateReb_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateReb_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateReb_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateReb != "") {
			selectLateReb += "_";
		}
		selectLateReb += numtxt;
	});
	p_selectLateReb = selectLateReb;

	var selectLateDis = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDis_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDis_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDis_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateDis != "") {
			selectLateDis += "_";
		}
		selectLateDis += numtxt;
	});
	p_selectLateDis = selectLateDis;

	var selectLateRebE22 = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateRebE22 != "") {
			selectLateRebE22 += "_";
		}
		selectLateRebE22 += numtxt;
	});
	p_selectLateRebE22 = selectLateRebE22;

	var selectLateDisE22 = "";
	$(".selectedEtcFeeClass" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateDisE22 != "") {
			selectLateDisE22 += "_";
		}
		selectLateDisE22 += numtxt;
	});
	p_selectLateDisE22 = selectLateDisE22;


	var selectLateRebETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateRebETC2 != "") {
			selectLateRebETC2 += "_";
		}
		selectLateRebETC2 += numtxt;
	});
	p_selectLateRebETC2 = selectLateRebETC2;

	var selectLateDisETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateDisETC2 != "") {
			selectLateDisETC2 += "_";
		}
		selectLateDisETC2 += numtxt;
	});
	p_selectLateDisETC2 = selectLateDisETC2;

	var selectLateRebE22ETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22ETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22ETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateRebE22ETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateRebE22ETC2 != "") {
			selectLateRebE22ETC2 += "_";
		}
		selectLateRebE22ETC2 += numtxt;
	});
	p_selectLateRebE22ETC2 = selectLateRebE22ETC2;

	var selectLateDisE22ETC2 = "";
	$(".selectedEtcFeeClassETC2" + selectedKeiroNumber).each( function() {

		var numtxt = "empty";

		var $tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22ETC2_" + selectedKeiroNumber).first();
		var $tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22ETC2_" + selectedKeiroNumber + ":eq(1)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		$tmpElem = $(this).closest(".js_etc_fee_area").find(".print_routeLateDisE22ETC2_" + selectedKeiroNumber + ":eq(2)");
		$tmpElem1 = $tmpElem.closest("dd").prev("dt").find("input[name^=etc_radio_]").first();
		if($tmpElem1.prop('checked') == true){
			numtxt = $tmpElem.text();
		}
		if(selectLateDisE22ETC2 != "") {
			selectLateDisE22ETC2 += "_";
		}
		selectLateDisE22ETC2 += numtxt;
	});
	p_selectLateDisE22ETC2 = selectLateDisE22ETC2;
	// V11.0.0 ADD END

	var $fm_print = $('<form />', {
            method: 'post',
            action: constDpPrint_List[drLangCode],
            target: '_print',
	    name:'formRoutePrint',
        });
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'etcFee',
            value: p_etcFee,
        }));
        $fm_print.append($('<input />', {
            type: 'hidden',
            name: 'etcFee2',
            value: p_etcFee2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectetc',
            value: p_selectetc,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectetc2',
            value: p_selectetc2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'finishReduce30',
            value: p_finishReduce30,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'finishReduce50',
            value: p_finishReduce50,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'finishReduce30ETC2',
            value: p_finishReduce30ETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'finishReduce50ETC2',
            value: p_finishReduce50ETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce30',
            value: p_selectReduce30,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce50',
            value: p_selectReduce50,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce30ETC2',
            value: p_selectReduce30ETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce50ETC2',
            value: p_selectReduce50ETC2,
        }));
	// V12.0.0 ADD START
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce30Dis',
            value: p_selectReduce30Dis,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce50Dis',
            value: p_selectReduce50Dis,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce30DisETC2',
            value: p_selectReduce30DisETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectReduce50DisETC2',
            value: p_selectReduce50DisETC2,
        }));
	// V12.0.0 ADD END

	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateReb',
            value: p_selectLateReb,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateDis',
            value: p_selectLateDis,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateRebE22',
            value: p_selectLateRebE22,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateDisE22',
            value: p_selectLateDisE22,
        }));

	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateRebETC2',
            value: p_selectLateRebETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateDisETC2',
            value: p_selectLateDisETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateRebE22ETC2',
            value: p_selectLateRebE22ETC2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'selectLateDisE22ETC2',
            value: p_selectLateDisE22ETC2,
        }));

	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'latenight',
            value: $('#formSearchmAgain input[name="latenight"]').val(),
        }));


	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'startPlaceCode',
            value: initStartIc,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'arrivePlaceCode',
            value: initEndIc,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlaceCode',
            value: initThroughIc,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlaceCode2',
            value: initThroughIc2,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlaceCode3',
            value: initThroughIc3,
        }));
	// V12.0.0 ADD START
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlaceCode4',
            value: initThroughIc4,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlaceCode5',
            value: initThroughIc5,
        }));
	// V12.0.0 ADD END

	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'carType',
            value: $('#formSearchmAgain select[name="carType"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'priority',
            value: $('#formSearchmAgain input[name="priority"]:checked').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'searchYear',
            value: $('#formSearchmAgain input[name="searchYear"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'searchMonth',
            value: $('#formSearchmAgain input[name="searchMonth"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'searchDay',
            value: $('#formSearchmAgain input[name="searchDay"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'searchHour',
            value: $('#formSearchmAgain select[name="searchHour"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'searchMinute',
            value: $('#formSearchmAgain select[name="searchMinute"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'kind',
            value: $('#formSearchmAgain input[name="kind"]:checked').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'routeNo',
            value: num-1,
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'roadType',
            value: $('#formSearch_follow input[name="roadType"]').val(),
        }));


	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'startPlace',
            value: $('#formSearchmAgain input[name="startPlaceKana"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'arrivePlace',
            value: $('#formSearchmAgain input[name="arrivePlaceKana"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlace',
            value: $('#formSearchmAgain input[name="keiyuPlaceKana"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlace2',
            value: $('#formSearchmAgain input[name="keiyuPlaceKana2"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlace3',
            value: $('#formSearchmAgain input[name="keiyuPlaceKana3"]').val(),
        }));
	// V12.0.0 ADD START
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlace4',
            value: $('#formSearchmAgain input[name="keiyuPlaceKana4"]').val(),
        }));
	$fm_print.append($('<input />', {
            type: 'hidden',
            name: 'keiyuPlace5',
            value: $('#formSearchmAgain input[name="keiyuPlaceKana5"]').val(),
        }));
	// V12.0.0 ADD END

        $fm_print.appendTo(document.body);
        $fm_print.submit();
        $fm_print.remove();
}
/*
function goPrint(num) {

	// ETC合計料金設定
	document.formSearchmAgain.etcFee.value = $("#fee_etc" + selectedKeiroNumber).text();

	// V3.0.0 MOD START
	// ETC2.0合計料金設定
	document.formSearchmAgain.etcFee2.value = $("#fee_etc2" + selectedKeiroNumber).text();

	// 選択されたETC割引タイプ取得
	var elems = document.getElementsByTagName("span");
	
	var name = "selectedEtcNoClass" + num;
	var name2 = "selectedEtcNoClassETC2" + num;

	var s_etc_param = "";
  	var s_etc_param2 = "";
  	
	var speNo = 1;
	var speNo2 = 1;
	for(var i = 0; i < elems.length; i++){
	    
		var classes = elems[i].className.split(" ");
	
		if(classes.indexOf(name) != -1){
			
			//s_etc_param += "&etcSpecial[" + speNo + "]=";

			if(speNo != 1){
				s_etc_param += "_";
			}

			s_etc_param += $(elems[i]).text();
			
			speNo++;
		}

		// ETC2.0
		if(classes.indexOf(name2) != -1){
			
			if(speNo2 != 1){
				s_etc_param2 += "_";
			}

			s_etc_param2 += $(elems[i]).text();
			
			speNo2++;
		}
	}

	document.formSearchmAgain.selectetc.value = s_etc_param;
	document.formSearchmAgain.selectetc2.value = s_etc_param2;

//alert(s_etc_param);
//alert(s_etc_param2);

	// 朝夕割引合計金額設定
	if($("#totalReduce30_" + selectedKeiroNumber).is(':visible')){
		document.formSearchmAgain.finishReduce30.value = $("#totalReduce30_" + selectedKeiroNumber).text();
	} else {
		document.formSearchmAgain.finishReduce30.value = "empty";
	}
	if($("#totalReduce50_" + selectedKeiroNumber).is(':visible')){
		document.formSearchmAgain.finishReduce50.value = $("#totalReduce50_" + selectedKeiroNumber).text();
	} else {
		document.formSearchmAgain.finishReduce50.value = "empty";
	}

	// 朝夕割引合計金額設定(ETC2.0)
	if($("#totalReduce30ETC2_" + selectedKeiroNumber).is(':visible')){
		document.formSearchmAgain.finishReduce30ETC2.value = $("#totalReduce30ETC2_" + selectedKeiroNumber).text();
	} else {
		document.formSearchmAgain.finishReduce30ETC2.value = "empty";
	}
	if($("#totalReduce50ETC2_" + selectedKeiroNumber).is(':visible')){
		document.formSearchmAgain.finishReduce50ETC2.value = $("#totalReduce50ETC2_" + selectedKeiroNumber).text();
	} else {
		document.formSearchmAgain.finishReduce50ETC2.value = "empty";
	}

	
	
	// 選択された朝夕割引30%還元値設定
	var selectReduce30 = "";
	for (var i=0; i<$(".routeReduce30_" + selectedKeiroNumber).length; i++) {
		var numtxt = "empty";
		if($(".routeReduce30_" + selectedKeiroNumber).eq(i).is(':visible')){
			numtxt = $(".routeReduce30_" + selectedKeiroNumber).eq(i).text();
		}
		if(i != 0) {
			selectReduce30 += "_";
		}
		selectReduce30 += numtxt;
	}
	document.formSearchmAgain.selectReduce30.value = selectReduce30;

	// 選択された朝夕割引50%還元値設定
	var selectReduce50 = "";
	for (var i=0; i<$(".routeReduce50_" + selectedKeiroNumber).length; i++) {
		var numtxt = "empty";
		if($(".routeReduce50_" + selectedKeiroNumber).eq(i).is(':visible')){
			numtxt = $(".routeReduce50_" + selectedKeiroNumber).eq(i).text();
		}
		if(i != 0) {
			selectReduce50 += "_";
		}
		selectReduce50 += numtxt;
	}
	document.formSearchmAgain.selectReduce50.value = selectReduce50;

	// 選択された朝夕割引30%還元値設定(ETC2.0)
	var selectReduce30ETC2 = "";
	for (var i=0; i<$(".routeReduce30ETC2_" + selectedKeiroNumber).length; i++) {
		var numtxt = "empty";
		if($(".routeReduce30ETC2_" + selectedKeiroNumber).eq(i).is(':visible')){
			numtxt = $(".routeReduce30ETC2_" + selectedKeiroNumber).eq(i).text();
		}
		if(i != 0) {
			selectReduce30ETC2 += "_";
		}
		selectReduce30ETC2 += numtxt;
	}
	document.formSearchmAgain.selectReduce30ETC2.value = selectReduce30ETC2;

	// 選択された朝夕割引50%還元値設定(ETC2.0)
	var selectReduce50ETC2 = "";
	for (var i=0; i<$(".routeReduce50ETC2_" + selectedKeiroNumber).length; i++) {
		var numtxt = "empty";
		if($(".routeReduce50ETC2_" + selectedKeiroNumber).eq(i).is(':visible')){
			numtxt = $(".routeReduce50ETC2_" + selectedKeiroNumber).eq(i).text();
		}
		if(i != 0) {
			selectReduce50ETC2 += "_";
		}
		selectReduce50ETC2 += numtxt;
	}
	document.formSearchmAgain.selectReduce50ETC2.value = selectReduce50ETC2;
	// V3.0.0 MOD END

	document.formSearchmAgain.target = "_print";
	document.formSearchmAgain.routeNo.value = num-1;
	document.formSearchmAgain.action = "/dp/RoutePrintNew";
	document.formSearchmAgain.submit();
}
*/

//----------------------------------------------
// ETC割引説明ページ表示(2014年4月消費税対応後)
//----------------------------------------------
function goEtcPageNew(type) {

	var url = "/";

	switch(type){
		//case 8:
		case 7:
			//url = "/etc/holiday_daytime_discount/holiday20140401.html";
			url = "//driveplaza.com/traffic/tolls_etc/etc_dis_weekend/";
			break;
		case 27:
			//url = "/etc/aqualine20140401.html";
			url = "//driveplaza.com/traffic/tolls_etc/etc_dis_aqualine/";
			break;
		case 33:
			//url = "/etc/night_discount/midnight20140401.html";
			url = "//driveplaza.com/traffic/tolls_etc/etc_dis_night/";
			break;
		case 30:
			url = "//driveplaza.com/etc/dis/etc_dis_aqualine_social_experiment/";
			break;
		case 31:
			url = "//driveplaza.com/etc/dis/etc_dis_aqualine_social_experiment/";
			break;
		case 51:
			url = "//driveplaza.com/etc/dis/etc_dis_night_fix/";
			break;
		case 501:
			url = "//driveplaza.com/traffic/tolls_etc/etc_dis_weekday/";
		default:
			break;
	}
/*
	document.formSearchmAgain.action=url;
	document.formSearchmAgain.target="etcwari";
	document.formSearchmAgain.submit();
*/

	window.open(url);

}




//----------------------------------------------
// 日付分割処理
//----------------------------------------------
function setHiddenDate(id) {
	//var searchDate = ($('.js_search_date').val()).split('/');
	var searchDate = ($('#' + id).val()).split('/');
	if(searchDate.length == 3){
		if(!isNaN(searchDate[0])){
			$(':hidden[name="searchYear"]').val(searchDate[0]);
		}
		if(!isNaN(searchDate[1])){
			$(':hidden[name="searchMonth"]').val(Number(searchDate[1])); // 先頭の0を取るためにNumber変換
		}
		if(!isNaN(searchDate[2])){
			$(':hidden[name="searchDay"]').val(Number(searchDate[2])); // 先頭の0を取るためにNumber変換
		}
	}
}

// URLのSearchパラメータ取得（マイルートから遷移してきた場合）
function getUrlSearchString()
{
	var result = {};
	// 最初の1文字 (?記号) を除いた文字列を取得する
	var query = window.location.search.substring( 1 );

	// クエリの区切り記号 (&) で文字列を配列に分割する
	var parameters = query.split( '&' );

	for( var i = 0; i < parameters.length; i++ ){
		// パラメータ名とパラメータ値に分割する
		var element = parameters[ i ].split( '=' );

		var paramName = decodeURIComponent( element[ 0 ] );
		var paramValue = decodeURIComponent( element[ 1 ] );

		// パラメータ名をキーとして連想配列に追加する
		result[ paramName ] = paramValue;
	}
	return result;
}

// APIにICコードを渡し、コードからIC名を同期通信(async:false)で取得
function searchIcFromCode(icCode){
	var tempICCode = icCode;
	var tempICName = "";
	var tempEncICCode = encodeURIComponent(tempICCode);
			
	$.ajax({
		url:"/community/icsearch_fromcode_api.php?val_iccode="+tempEncICCode,
				
		async:false,
		cache:false,
		dataType:"xml",
		error:function(){
			//alert("IC search error");
			console.log("IC search error");
		},
		success:function(xml){
			$(xml).find('IcItem').each(function(i){
				var searchedICCode = escapeHTML($(this).find('Code').text());
				if(tempICCode == searchedICCode){
					tempICName = escapeHTML($(this).find('Name').text());
					return false; //ループを抜ける
				}
			});
		}
	});

	return(tempICName);
}

// 道路から選ぶhtml取得
function getRoadSearchHtml(){

	
			
	$.ajax({
		//url:"/dp/SearchTop",
		url:constDpSearchTop_List[drLangCode],
		async:true,
		cache:false,
		dataType:"html",
		error:function(){
			$('#js_search_road_conteiner').html('no data');
		},
		success:function(data){
			var html = $(data).find('#js_search_road_target')[0].outerHTML;
			//XSS
			html = $.parseHTML(html);

			$('#js_search_road_conteiner').html(html);

			// 初期選択状態設定
			$('.box-roadLevel02, .box-roadLevel03').hide();
		}
	});
}

// SAPAアイコンデータ設定
function getSAPAServiceIcon(){

	$.ajax({
		url:"/jsp/portal/route/js/sapa_icon_data.json",
		async:true,
		//cache:false,
		cache:true,
		dataType:"json",
		error:function(){
			console.log('sapa_icon_data get error');
		},
		success:function(sapa_icon_data){
			

	$('a.js_sapalink').each(function() {

		var tmpHref = $(this).attr('href');
		var tmpHrefArray = tmpHref.split("/"); //スラッシュで区切る
		if(tmpHrefArray.length > 6){
			var tmpSAPAcode = tmpHrefArray[5] + tmpHrefArray[6];
			var tmpIconData = sapa_icon_data[tmpSAPAcode];
			if(tmpIconData){
				var tmpIconDataArray = tmpIconData.split(",");
				if(tmpIconDataArray){
					var tmpUl = $(this).closest('.js_sapa_icon_area').children('ul.li-shisetsuIcon');
					for( var i = 0; i < tmpIconDataArray.length; i++ ){

						var tmpId;
						if(i < 9){
							tmpId = "0" + (i +1);
						}else{
							tmpId = i +1;
						}

						var tmpColor = "gray";
						if(tmpIconDataArray[i] && tmpIconDataArray[i] == 1){
							tmpColor = "green";
						}
						tmpUl.append('<li><img src="/assets/img/common/icon_shisetsu_' + tmpColor + '_' + tmpId + '.svg" alt=""></li>');
					}

					//var tmpUl = $(this).closest('.js_sapa_icon_area').children('ul.li-shisetsuIcon');
					//tmpUl.children('li').each(function() {
					//$('li img', tmpUl).each(function(index) {
 	   				//	if(tmpIconDataArray[index] && tmpIconDataArray[index] == 1){
					//		var imgName = $(this).attr('src');
					//		imgName = imgName.replace('gray','green');
					//		$(this).attr('src',imgName);
					//	}
					//});
				}
			}else{
				// データがない、中西の場合
				var tmpUl = $(this).closest('.js_sapa_icon_area').children('ul.li-shisetsuIcon');
				$(tmpUl).hide();
				$(tmpUl).next('p').hide();
				//tmpUl.text("施設の詳しい情報は、NEXCO中日本、NEXCO西日本（西日本高速道路サービス・ホールディングス）のHPをご覧下さい。");
			}
		}
		
		//var sapaCode = tmpHref.match(/\d{2,7}/);

	});


		}
	});

}

// 注意喚起アコーディオンオープン
function openAttention(){
	if(!$('dl.caution').hasClass('is-open')){
		$('#seamless_notes').click();
	}
}

// 深夜割引計算APIコール
function searchLatenight(keiroareaid){

	var param = "lt_carType=";
	param += $('#formSearchmAgain select[name="carType"]').val();
	if ($(".ltInArea #lt_chk_bus_" + keiroareaid + ":checked").val() == "on") {
		param += "&lt_chk_bus=on";
	}
	$('.ltInArea input[type=text].lt_dstdisc_' + keiroareaid).each(function() {
        	var val = $(this).val();
		val = encodeURIComponent(val);
		var paramname = $(this).data('paramname');
		var add_dist = $("#" + keiroareaid + "_" + paramname + "_2 input[type=text]").val();
		if(add_dist != ""){
			val = val + "," + encodeURIComponent(add_dist);
		}
		var paramline = "&" + paramname + "=" + val;
		param += paramline;
    	});
	$('span.lt_param_' + keiroareaid).each(function() {
        	var val = $(this).text();
		if(val != ""){
			var paramname = $(this).data('paramname');
			var paramline = "&" + paramname + "=" + val;
			param += paramline;
		}
    	});

	param += "&lt_lang=" + drLangCode;
	
	// 追加チェックメッセージ取得
	var valkukanno_before ="";
	var val_chkprm = "";
	var counter = 0;
	//$('input.lt_chk_add_' + keiroareaid).each(function() {
	$('.lt_chk_add_' + keiroareaid).each(function() {
		var valkukanno = $(this).data('valkukanno');

		var valaddcode = $(this).data('valaddcode');


		if(valkukanno_before != valkukanno){
			val_chkprm += "&lt_chkprm_" + valkukanno + "=" ;
			counter = 0;
		}

		
		if(counter != 0){
			val_chkprm += ",";
		}

		if(valaddcode == "2"){
			val_chkprm += $(this).val();
		}else{
			if($(this).prop("checked")) {
				val_chkprm += "1";
			}else{
				val_chkprm += "0";
			}
		}

		valkukanno_before = valkukanno;

		counter++;
    	});
	param += val_chkprm;
	//alert("val_chkprm:"+val_chkprm);
	
	var counter_kukan = 0;

	var select_etcno = $("#select_etcno_" + keiroareaid).text();
	
	$.ajax({
		//url:"/dp/LatenightAPI?" + param,
		url:"/dp/LatenightAPI",
				
		async:false,
		cache:false,
		//type: "GET",
		type: "POST",
		data:param,
		//processData:false,
		dataType:"json",
		error:function(){
			console.log("Late night search error");

			$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text("API ACCESS ERROR");

			$(".ltNormalArea.keiroAreaId_" + keiroareaid).hide();
			$(".ltErrArea.keiroAreaId_" + keiroareaid).show();
		},
		success:function(json){
			//console.log(json);
			if(json.errorType){
				if(json.errorType == "3"){
					$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text("API RETURN ERROR");
				}else{
					if(json.errorType == "11"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List1[drLangCode]);
					}else if(json.errorType == "12"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List2[drLangCode]);
					}else if(json.errorType == "13"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List3[drLangCode]);
					}else if(json.errorType == "14"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List4[drLangCode]);
					}else if(json.errorType == "15"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List5[drLangCode]);
					}else if(json.errorType == "16"){
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(constLNErrMsg_List6[drLangCode]);
					}else{
						$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text("API RETURN ERROR2");
					}
					//$('.ltErrArea.keiroAreaId_' + keiroareaid + ' .ltErrMsg').text(json.errorMsg);
				}
				$(".ltNormalArea.keiroAreaId_" + keiroareaid).hide();
				$(".ltErrArea.keiroAreaId_" + keiroareaid).show();
			}else{
				for( var i = 0; i < json.length; i++ ){
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_DD_' + counter_kukan).text(json[i].discountdist);
				//$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_EtcChg_' + counter_kukan).text(json[i].etccharge);
				//$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_Etc2Chg_' + counter_kukan).text(json[i].etc2charge);

				if(select_etcno == "1"){
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND_' + counter_kukan).text(json[i].ltwdiscount1);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND2_' + counter_kukan).text(json[i].lthdiscount1);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR_' + counter_kukan).text(json[i].ltwrebate1);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR2_' + counter_kukan).text(json[i].lthrebate1);

				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND-exit22_' + counter_kukan).text(json[i].ltwdiscount1e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND2-exit22_' + counter_kukan).text(json[i].lthdiscount1e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR-exit22_' + counter_kukan).text(json[i].ltwrebate1e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR2-exit22_' + counter_kukan).text(json[i].lthrebate1e2);

				$(".ltOutArea.keiroAreaId_" + keiroareaid + ' .lt_ECCH2').hide();
				$(".ltOutArea.keiroAreaId_" + keiroareaid + ' .lt_ECCH').show();

				}else{
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND_' + counter_kukan).text(json[i].ltwdiscount2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND2_' + counter_kukan).text(json[i].lthdiscount2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR_' + counter_kukan).text(json[i].ltwrebate2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR2_' + counter_kukan).text(json[i].lthrebate2);

				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND-exit22_' + counter_kukan).text(json[i].ltwdiscount2e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LND2-exit22_' + counter_kukan).text(json[i].lthdiscount2e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR-exit22_' + counter_kukan).text(json[i].ltwrebate2e2);
				$('.ltOutArea.keiroAreaId_' + keiroareaid + ' .lt_LNR2-exit22_' + counter_kukan).text(json[i].lthrebate2e2);

				$(".ltOutArea.keiroAreaId_" + keiroareaid + ' .lt_ECCH2').show();
				$(".ltOutArea.keiroAreaId_" + keiroareaid + ' .lt_ECCH').hide();
				}

				counter_kukan++;
        			}

				$(".ltNormalArea.keiroAreaId_" + keiroareaid).show();
				$(".ltErrArea.keiroAreaId_" + keiroareaid).hide();
			}
		}
	});
}