//var suggestIC_stSelectFlg = false;	// 出発ICがサジェストから選択されたかのフラグ
//var suggestIC_arSelectFlg = false;	// 到着ICがサジェストから選択されたかのフラグ

$(document).ready(function() {

	$.ui.autocomplete.prototype._renderItem = function(ul, item) {
		return $("<li></li>").data("item.autocomplete", item).append($("<a></a>").html(item.label)).appendTo(ul);
	};

	//$("input[name=startPlaceKana], input[name=arrivePlaceKana], input[name=keiyuPlaceKana], input[name=keiyuPlaceKana2], input[name=keiyuPlaceKana3]").autocomplete({
	//$("input[name=startPlaceKana], input[name=arrivePlaceKana]").autocomplete({
	$("input[name=startPlaceKana]").autocomplete({
	//$("input[name=startPlaceKana], input[name=arrivePlaceKana], .js-waypoint1ICField, input[name=keiyuPlaceKana2], input[name=keiyuPlaceKana3]").autocomplete({
	//$("input[name=startPlaceKana], input[name=arrivePlaceKana], .js-departureICField_follow, input[name=keiyuPlaceKana2], input[name=keiyuPlaceKana3]").autocomplete({
		source: function(request, response) {
		
			// 正規表現オブジェクトを生成
                        var searchWord = request.term;
			searchWord = encodeURIComponent(searchWord);

			$.ajax({
				//url:"/community/icsearch_api.php?val_word="+searchWord,
				url:"/community/icsearch_api.php?ic_type=start&val_word="+searchWord,
				async:true,
				cache:false,
				dataType:"xml",
				error:function(){
				},
				success:function(xml){
					var list = [];
					$(xml).find('IcItem').each(function(i){
						var icName = escapeHTML($(this).find('Name').text());
						var roadName = escapeHTML($(this).find('RoadName').text());
						//list.push(icName);
						list.push({
							label : icName + "<span style='color:grey;font-size:x-small;float:right;'>" + roadName + "</span>",
							value : icName
						});
					});
					response(list);
				}
			});

		},
		search: function( event, ui ) {
/*
			var textName = $(this).attr("name");
			if(textName == "startPlaceKana"){
				suggestIC_stSelectFlg = false;
			}
			if(textName == "arrivePlaceKana"){
				suggestIC_arSelectFlg = false;
			}
			if(textName == "keiyuPlaceKana"){
				$("input[name=k1suggest]").val("false");
			}
			if(textName == "keiyuPlaceKana2"){
				$("input[name=k2suggest]").val("false");
			}
			if(textName == "keiyuPlaceKana3"){
				$("input[name=k3suggest]").val("false");
			}
			$("input[name=startArrive]").val("false");
*/
		},
		select: function( event, ui ) {
			//$("input[name=startPlaceKana], input[name=arrivePlaceKana], input[name=keiyuPlaceKana], input[name=keiyuPlaceKana2], input[name=keiyuPlaceKana3]").blur();
			$("input[name=startPlaceKana], input[name=arrivePlaceKana]").blur();
/*			
			var textName = $(this).attr("name");
			if(textName == "startPlaceKana"){
				suggestIC_stSelectFlg = true;
			}
			if(textName == "arrivePlaceKana"){
				suggestIC_arSelectFlg = true;
			}

			if(suggestIC_stSelectFlg == true && suggestIC_arSelectFlg == true){
				$("input[name=startArrive]").val("true");
			}
			if(textName == "keiyuPlaceKana"){
				$("input[name=k1suggest]").val("true");
			}
			if(textName == "keiyuPlaceKana2"){
				$("input[name=k2suggest]").val("true");
			}
			if(textName == "keiyuPlaceKana3"){
				$("input[name=k3suggest]").val("true");
			}
*/
		},
		open: function( event, ui ) {
			window.currentAutocomplete=true;
			//console.log("window.currentAutocomplete=true;");
		},
		close: function( event, ui ) {
			window.currentAutocomplete=false;
			//console.log("window.currentAutocomplete=false;");
		},
		delay:0,
		minLength:1
	});

	$("input[name=arrivePlaceKana]").autocomplete({
		source: function(request, response) {
		
			// 正規表現オブジェクトを生成
                        var searchWord = request.term;
			searchWord = encodeURIComponent(searchWord);

			$.ajax({
				url:"/community/icsearch_api.php?ic_type=arrive&val_word="+searchWord,
				async:true,
				cache:false,
				dataType:"xml",
				error:function(){
				},
				success:function(xml){
					var list = [];
					$(xml).find('IcItem').each(function(i){
						var icName = escapeHTML($(this).find('Name').text());
						var roadName = escapeHTML($(this).find('RoadName').text());
						//list.push(icName);
						list.push({
							label : icName + "<span style='color:grey;font-size:x-small;float:right;'>" + roadName + "</span>",
							value : icName
						});
					});
					response(list);
				}
			});

		},
		search: function( event, ui ) {
		},
		select: function( event, ui ) {
			$("input[name=startPlaceKana], input[name=arrivePlaceKana]").blur();
		},
		open: function( event, ui ) {
			window.currentAutocomplete=true;
		},
		close: function( event, ui ) {
			window.currentAutocomplete=false;
		},
		delay:0,
		minLength:1
	});
});

function escapeHTML(str) {
  return str.replace(/[&"<>]/g, function(c) {
    return {
      "&": "&amp;",
      '"': "&quot;",
      "<": "&lt;",
      ">": "&gt;"
    }[c];
  });
}

