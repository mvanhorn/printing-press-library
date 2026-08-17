# SculptOK api-open — verified contract (rendered from www.sculptok.com/api/apidoc)

Base URL: https://api.sculptok.com/api-open/
Auth header: apikey: <key>  (Content-Type: application/json)
Envelope: {code, msg, data}; HTTP always 200. code 0 = success; 401 Unauthorized; 10020 apikey empty; 10021 invalid key.
Operator: MODERNFOX LTD. Credits shared with web app. ULTIMATE != unlimited API.

## Endpoints
1. GET  /point/info        -> data.point (number). No params.
2. GET  /point/page        params: limit(opt num), page(opt str). data.total, data.list[]{id,userId,actionType,remainValue,changeNum,remarks,createDate}
3. POST /image/upload      multipart/form-data field `file` (required). data.src (string url). [FREE]
4. POST /draw/prompt       JSON. imageUrl(req), style(normal|portrait|sketch|pro=normal), hd_fix(auto|manual=manual), optimal_size(true|false=true), extInfo(false|8bit|16bit|exr=false; exr pro-only), version(1.0|1.5, pro-only=1.0), draw_hd(2k|4k, pro-only=2k). -> data.promptId. Cost 10 / pro2k 15 / pro4k 30.
5. POST /draw/hd/prompt    JSON. imageUrl(req), hdFix(true|false), removeBack(anime|general; omit=skip). -> data.promptId. Cost 2.
6. POST /draw/3d/prompt    JSON. imageUrl(req), hd_fix(basic|standard|high=basic). -> data.promptId. Cost 10.
7. POST /draw/stl/prompt   JSON. image_url(req), width_mm(40-240=120), min_thickness(0.4-8=1.6), max_thickness(0.4-25=5.0), invert(bool=false), scale_image(0-100=50). -> data.promptId. Cost 3.
8. GET  /draw/prompt       params: uuid(req)=promptId. data{id,currentStep,status(num),createDate,userId,upImageUrl,promptId,imgRecords[3],position}. (Shared path with #4, GET vs POST.)
9. GET  /image/page        params: limit(opt num), page(opt num). data.total, data.list[]{id,userId,imgUrl,createDate}

## Casing nuances (DO NOT NORMALIZE)
- depth-map: imageUrl, hd_fix, optimal_size, extInfo, version, draw_hd
- bg/hd:     imageUrl, hdFix, removeBack
- 3d:        imageUrl, hd_fix
- stl:       image_url, width_mm, min_thickness, max_thickness, invert, scale_image
- status:    query uuid

## Pricing (credits): depth 10/15/30, bg+hd 2, 3d 10, stl 3. Reads (point/info, point/page, image/page, status) FREE.
