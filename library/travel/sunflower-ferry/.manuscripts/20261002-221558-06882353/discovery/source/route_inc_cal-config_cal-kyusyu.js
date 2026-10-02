    'use strict';

    document.addEventListener('DOMContentLoaded', () => {
      const calendarEl3 = document.getElementById('calendar3');
      const calendarEl4 = document.getElementById('calendar4');
      //const DAY_NAMES3 = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
      //const DAY_NAMES4 = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
      const calendarEvents3 = [
/***********関西発*********************************************/
 {title: 'A', start: '2026-10-01'},
 {title: 'A', start: '2026-10-02'},
 {title: 'A', start: '2026-10-03'},
 {title: 'A', start: '2026-10-04'},
 {title: 'A', start: '2026-10-05'},
 {title: 'A', start: '2026-10-06'},
 {title: 'A', start: '2026-10-07'},
 {title: 'A', start: '2026-10-08'},
 {title: 'B', start: '2026-10-09'},
 {title: 'B', start: '2026-10-10'},
 {title: 'A', start: '2026-10-11'},
 {title: 'A', start: '2026-10-12'},
 {title: 'A', start: '2026-10-13'},
 {title: 'A', start: '2026-10-14'},
 {title: 'A', start: '2026-10-15'},
 {title: 'A', start: '2026-10-16'},
 {title: 'A', start: '2026-10-17'},
 {title: 'A', start: '2026-10-18'},
 {title: 'A', start: '2026-10-19'},
 {title: 'A', start: '2026-10-20'},
 {title: 'A', start: '2026-10-21'},
 {title: 'A', start: '2026-10-22'},
 {title: 'A', start: '2026-10-23'},
 {title: 'A', start: '2026-10-24'},
 {title: 'A', start: '2026-10-25'},
 {title: 'A', start: '2026-10-26'},
 {title: 'A', start: '2026-10-27'},
 {title: 'A', start: '2026-10-28'},
 {title: 'A', start: '2026-10-29'},
 {title: 'A', start: '2026-10-30'},
 {title: 'A', start: '2026-10-31'},
 {title: 'A', start: '2026-11-01'},
 {title: 'A', start: '2026-11-02'},
 {title: 'A', start: '2026-11-03'},
 {title: 'A', start: '2026-11-04'},
 {title: 'A', start: '2026-11-05'},
 {title: 'A', start: '2026-11-06'},
 {title: 'A', start: '2026-11-07'},
 {title: 'A', start: '2026-11-08'},
 {title: 'A', start: '2026-11-09'},
 {title: 'A', start: '2026-11-10'},
 {title: 'A', start: '2026-11-11'},
 {title: 'A', start: '2026-11-12'},
 {title: 'A', start: '2026-11-13'},
 {title: 'A', start: '2026-11-14'},
 {title: 'A', start: '2026-11-15'},
 {title: 'A', start: '2026-11-16'},
 {title: 'A', start: '2026-11-17'},
 {title: 'A', start: '2026-11-18'},
 {title: 'A', start: '2026-11-19'},
 {title: 'B', start: '2026-11-20'},
 {title: 'B', start: '2026-11-21'},
 {title: 'A', start: '2026-11-22'},
 {title: 'A', start: '2026-11-23'},
 {title: 'A', start: '2026-11-24'},
 {title: 'A', start: '2026-11-25'},
 {title: 'A', start: '2026-11-26'},
 {title: 'A', start: '2026-11-27'},
 {title: 'A', start: '2026-11-28'},
 {title: 'A', start: '2026-11-29'},
 {title: 'A', start: '2026-11-30'},
 {title: 'A', start: '2026-12-01'},
 {title: 'A', start: '2026-12-02'},
 {title: 'A', start: '2026-12-03'},
 {title: 'A', start: '2026-12-04'},
 {title: 'A', start: '2026-12-05'},
 {title: 'A', start: '2026-12-06'},
 {title: 'A', start: '2026-12-07'},
 {title: 'A', start: '2026-12-08'},
 {title: 'A', start: '2026-12-09'},
 {title: 'A', start: '2026-12-10'},
 {title: 'A', start: '2026-12-11'},
 {title: 'A', start: '2026-12-12'},
 {title: 'A', start: '2026-12-13'},
 {title: 'A', start: '2026-12-14'},
 {title: 'A', start: '2026-12-15'},
 {title: 'A', start: '2026-12-16'},
 {title: 'A', start: '2026-12-17'},
 {title: 'A', start: '2026-12-18'},
 {title: 'A', start: '2026-12-19'},
 {title: 'A', start: '2026-12-20'},
 {title: 'A', start: '2026-12-21'},
 {title: 'A', start: '2026-12-22'},
 {title: 'A', start: '2026-12-23'},
 {title: 'A', start: '2026-12-24'},
 {title: 'B', start: '2026-12-25'},
 {title: 'B', start: '2026-12-26'},
 {title: 'B', start: '2026-12-27'},
 {title: 'B', start: '2026-12-28'},
 {title: 'C', start: '2026-12-29'},
 {title: 'C', start: '2026-12-30'},
 {title: 'B', start: '2026-12-31'},
 {title: 'B', start: '2027-01-01'},
 {title: 'B', start: '2027-01-02'},
 {title: 'B', start: '2027-01-03'},
 {title: 'A', start: '2027-01-04'},
 {title: 'A', start: '2027-01-05'},
 {title: 'A', start: '2027-01-06'},
 {title: 'A', start: '2027-01-07'},
 {title: 'B', start: '2027-01-08'},
 {title: 'B', start: '2027-01-09'},
 {title: 'A', start: '2027-01-10'},
 {title: 'A', start: '2027-01-11'},
 {title: 'A', start: '2027-01-12'},
 {title: 'A', start: '2027-01-13'},
 {title: 'A', start: '2027-01-14'},
 {title: 'A', start: '2027-01-15'},
 {title: 'A', start: '2027-01-16'},
 {title: 'A', start: '2027-01-17'},
 {title: 'A', start: '2027-01-18'},
 {title: 'A', start: '2027-01-19'},
 {title: 'A', start: '2027-01-20'},
 {title: 'A', start: '2027-01-21'},
 {title: 'A', start: '2027-01-22'},
 {title: 'A', start: '2027-01-23'},
 {title: 'A', start: '2027-01-24'},
 {title: 'A', start: '2027-01-25'},
 {title: 'A', start: '2027-01-26'},
 {title: 'A', start: '2027-01-27'},
 {title: 'A', start: '2027-01-28'},
 {title: 'A', start: '2027-01-29'},
 {title: 'A', start: '2027-01-30'},
 {title: 'A', start: '2027-01-31'},
 {title: 'A', start: '2027-02-01'},
 {title: 'A', start: '2027-02-02'},
 {title: 'A', start: '2027-02-03'},
 {title: 'A', start: '2027-02-04'},
 {title: 'A', start: '2027-02-05'},
 {title: 'A', start: '2027-02-06'},
 {title: 'A', start: '2027-02-07'},
 {title: 'A', start: '2027-02-08'},
 {title: 'A', start: '2027-02-09'},
 {title: 'A', start: '2027-02-10'},
 {title: 'A', start: '2027-02-11'},
 {title: 'A', start: '2027-02-12'},
 {title: 'A', start: '2027-02-13'},
 {title: 'A', start: '2027-02-14'},
 {title: 'A', start: '2027-02-15'},
 {title: 'A', start: '2027-02-16'},
 {title: 'A', start: '2027-02-17'},
 {title: 'A', start: '2027-02-18'},
 {title: 'A', start: '2027-02-19'},
 {title: 'A', start: '2027-02-20'},
 {title: 'A', start: '2027-02-21'},
 {title: 'A', start: '2027-02-22'},
 {title: 'A', start: '2027-02-23'},
 {title: 'A', start: '2027-02-24'},
 {title: 'A', start: '2027-02-25'},
 {title: 'A', start: '2027-02-26'},
 {title: 'A', start: '2027-02-27'},
 {title: 'A', start: '2027-02-28'},
 {title: 'A', start: '2027-03-01'},
 {title: 'A', start: '2027-03-02'},
 {title: 'A', start: '2027-03-03'},
 {title: 'A', start: '2027-03-04'},
 {title: 'A', start: '2027-03-05'},
 {title: 'A', start: '2027-03-06'},
 {title: 'A', start: '2027-03-07'},
 {title: 'A', start: '2027-03-08'},
 {title: 'A', start: '2027-03-09'},
 {title: 'A', start: '2027-03-10'},
 {title: 'A', start: '2027-03-11'},
 {title: 'A', start: '2027-03-12'},
 {title: 'A', start: '2027-03-13'},
 {title: 'A', start: '2027-03-14'},
 {title: 'A', start: '2027-03-15'},
 {title: 'A', start: '2027-03-16'},
 {title: 'A', start: '2027-03-17'},
 {title: 'B', start: '2027-03-18'},
 {title: 'B', start: '2027-03-19'},
 {title: 'B', start: '2027-03-20'},
 {title: 'B', start: '2027-03-21'},
 {title: 'B', start: '2027-03-22'},
 {title: 'B', start: '2027-03-23'},
 {title: 'B', start: '2027-03-24'},
 {title: 'B', start: '2027-03-25'},
 {title: 'B', start: '2027-03-26'},
 {title: 'B', start: '2027-03-27'},
 {title: 'B', start: '2027-03-28'},
 {title: 'B', start: '2027-03-29'},
 {title: 'B', start: '2027-03-30'},
 {title: 'B', start: '2027-03-31'},
/************関西発ここまで********************************************/
      ];
      const calendarEvents4 = [
/**********九州発**********************************************/
 {title: 'A', start: '2026-10-01'},
 {title: 'A', start: '2026-10-02'},
 {title: 'A', start: '2026-10-03'},
 {title: 'A', start: '2026-10-04'},
 {title: 'A', start: '2026-10-05'},
 {title: 'A', start: '2026-10-06'},
 {title: 'A', start: '2026-10-07'},
 {title: 'A', start: '2026-10-08'},
 {title: 'A', start: '2026-10-09'},
 {title: 'B', start: '2026-10-10'},
 {title: 'B', start: '2026-10-11'},
 {title: 'A', start: '2026-10-12'},
 {title: 'A', start: '2026-10-13'},
 {title: 'A', start: '2026-10-14'},
 {title: 'A', start: '2026-10-15'},
 {title: 'A', start: '2026-10-16'},
 {title: 'A', start: '2026-10-17'},
 {title: 'A', start: '2026-10-18'},
 {title: 'A', start: '2026-10-19'},
 {title: 'A', start: '2026-10-20'},
 {title: 'A', start: '2026-10-21'},
 {title: 'A', start: '2026-10-22'},
 {title: 'A', start: '2026-10-23'},
 {title: 'A', start: '2026-10-24'},
 {title: 'A', start: '2026-10-25'},
 {title: 'A', start: '2026-10-26'},
 {title: 'A', start: '2026-10-27'},
 {title: 'A', start: '2026-10-28'},
 {title: 'A', start: '2026-10-29'},
 {title: 'A', start: '2026-10-30'},
 {title: 'A', start: '2026-10-31'},
 {title: 'A', start: '2026-11-01'},
 {title: 'A', start: '2026-11-02'},
 {title: 'A', start: '2026-11-03'},
 {title: 'A', start: '2026-11-04'},
 {title: 'A', start: '2026-11-05'},
 {title: 'A', start: '2026-11-06'},
 {title: 'A', start: '2026-11-07'},
 {title: 'A', start: '2026-11-08'},
 {title: 'A', start: '2026-11-09'},
 {title: 'A', start: '2026-11-10'},
 {title: 'A', start: '2026-11-11'},
 {title: 'A', start: '2026-11-12'},
 {title: 'A', start: '2026-11-13'},
 {title: 'A', start: '2026-11-14'},
 {title: 'A', start: '2026-11-15'},
 {title: 'A', start: '2026-11-16'},
 {title: 'A', start: '2026-11-17'},
 {title: 'A', start: '2026-11-18'},
 {title: 'A', start: '2026-11-19'},
 {title: 'A', start: '2026-11-20'},
 {title: 'B', start: '2026-11-21'},
 {title: 'B', start: '2026-11-22'},
 {title: 'A', start: '2026-11-23'},
 {title: 'A', start: '2026-11-24'},
 {title: 'A', start: '2026-11-25'},
 {title: 'A', start: '2026-11-26'},
 {title: 'A', start: '2026-11-27'},
 {title: 'A', start: '2026-11-28'},
 {title: 'A', start: '2026-11-29'},
 {title: 'A', start: '2026-11-30'},
 {title: 'A', start: '2026-12-01'},
 {title: 'A', start: '2026-12-02'},
 {title: 'A', start: '2026-12-03'},
 {title: 'A', start: '2026-12-04'},
 {title: 'A', start: '2026-12-05'},
 {title: 'A', start: '2026-12-06'},
 {title: 'A', start: '2026-12-07'},
 {title: 'A', start: '2026-12-08'},
 {title: 'A', start: '2026-12-09'},
 {title: 'A', start: '2026-12-10'},
 {title: 'A', start: '2026-12-11'},
 {title: 'A', start: '2026-12-12'},
 {title: 'A', start: '2026-12-13'},
 {title: 'A', start: '2026-12-14'},
 {title: 'A', start: '2026-12-15'},
 {title: 'A', start: '2026-12-16'},
 {title: 'A', start: '2026-12-17'},
 {title: 'A', start: '2026-12-18'},
 {title: 'A', start: '2026-12-19'},
 {title: 'A', start: '2026-12-20'},
 {title: 'A', start: '2026-12-21'},
 {title: 'A', start: '2026-12-22'},
 {title: 'A', start: '2026-12-23'},
 {title: 'A', start: '2026-12-24'},
 {title: 'A', start: '2026-12-25'},
 {title: 'A', start: '2026-12-26'},
 {title: 'B', start: '2026-12-27'},
 {title: 'B', start: '2026-12-28'},
 {title: 'B', start: '2026-12-29'},
 {title: 'B', start: '2026-12-30'},
 {title: 'B', start: '2026-12-31'},
 {title: 'B', start: '2027-01-01'},
 {title: 'C', start: '2027-01-02'},
 {title: 'C', start: '2027-01-03'},
 {title: 'B', start: '2027-01-04'},
 {title: 'A', start: '2027-01-05'},
 {title: 'A', start: '2027-01-06'},
 {title: 'A', start: '2027-01-07'},
 {title: 'A', start: '2027-01-08'},
 {title: 'B', start: '2027-01-09'},
 {title: 'B', start: '2027-01-10'},
 {title: 'A', start: '2027-01-11'},
 {title: 'A', start: '2027-01-12'},
 {title: 'A', start: '2027-01-13'},
 {title: 'A', start: '2027-01-14'},
 {title: 'A', start: '2027-01-15'},
 {title: 'A', start: '2027-01-16'},
 {title: 'A', start: '2027-01-17'},
 {title: 'A', start: '2027-01-18'},
 {title: 'A', start: '2027-01-19'},
 {title: 'A', start: '2027-01-20'},
 {title: 'A', start: '2027-01-21'},
 {title: 'A', start: '2027-01-22'},
 {title: 'A', start: '2027-01-23'},
 {title: 'A', start: '2027-01-24'},
 {title: 'A', start: '2027-01-25'},
 {title: 'A', start: '2027-01-26'},
 {title: 'A', start: '2027-01-27'},
 {title: 'A', start: '2027-01-28'},
 {title: 'A', start: '2027-01-29'},
 {title: 'A', start: '2027-01-30'},
 {title: 'A', start: '2027-01-31'},
 {title: 'A', start: '2027-02-01'},
 {title: 'A', start: '2027-02-02'},
 {title: 'A', start: '2027-02-03'},
 {title: 'A', start: '2027-02-04'},
 {title: 'A', start: '2027-02-05'},
 {title: 'A', start: '2027-02-06'},
 {title: 'A', start: '2027-02-07'},
 {title: 'A', start: '2027-02-08'},
 {title: 'A', start: '2027-02-09'},
 {title: 'A', start: '2027-02-10'},
 {title: 'A', start: '2027-02-11'},
 {title: 'A', start: '2027-02-12'},
 {title: 'A', start: '2027-02-13'},
 {title: 'A', start: '2027-02-14'},
 {title: 'A', start: '2027-02-15'},
 {title: 'A', start: '2027-02-16'},
 {title: 'A', start: '2027-02-17'},
 {title: 'A', start: '2027-02-18'},
 {title: 'A', start: '2027-02-19'},
 {title: 'A', start: '2027-02-20'},
 {title: 'A', start: '2027-02-21'},
 {title: 'A', start: '2027-02-22'},
 {title: 'A', start: '2027-02-23'},
 {title: 'A', start: '2027-02-24'},
 {title: 'A', start: '2027-02-25'},
 {title: 'A', start: '2027-02-26'},
 {title: 'A', start: '2027-02-27'},
 {title: 'A', start: '2027-02-28'},
 {title: 'A', start: '2027-03-01'},
 {title: 'A', start: '2027-03-02'},
 {title: 'A', start: '2027-03-03'},
 {title: 'A', start: '2027-03-04'},
 {title: 'A', start: '2027-03-05'},
 {title: 'A', start: '2027-03-06'},
 {title: 'A', start: '2027-03-07'},
 {title: 'A', start: '2027-03-08'},
 {title: 'A', start: '2027-03-09'},
 {title: 'A', start: '2027-03-10'},
 {title: 'A', start: '2027-03-11'},
 {title: 'A', start: '2027-03-12'},
 {title: 'A', start: '2027-03-13'},
 {title: 'A', start: '2027-03-14'},
 {title: 'A', start: '2027-03-15'},
 {title: 'A', start: '2027-03-16'},
 {title: 'A', start: '2027-03-17'},
 {title: 'A', start: '2027-03-18'},
 {title: 'B', start: '2027-03-19'},
 {title: 'B', start: '2027-03-20'},
 {title: 'B', start: '2027-03-21'},
 {title: 'B', start: '2027-03-22'},
 {title: 'B', start: '2027-03-23'},
 {title: 'B', start: '2027-03-24'},
 {title: 'B', start: '2027-03-25'},
 {title: 'B', start: '2027-03-26'},
 {title: 'B', start: '2027-03-27'},
 {title: 'B', start: '2027-03-28'},
 {title: 'B', start: '2027-03-29'},
 {title: 'B', start: '2027-03-30'},
 {title: 'B', start: '2027-03-31'},
 /*************九州発ここまで*******************************************/
      ];
      const calendar3 = new FullCalendar.Calendar(calendarEl3, {
        initialView: 'dayGridMonth',
        timeZone: "Asia/Tokyo",
        height: "auto",
        validRange: {
          start: '2026-10-01',
          end: '2027-04-01'
        },
        events: calendarEvents3,
        dayCellContent: function (arg) {
          return arg.date.getDate();
        },
        navLinks: false,
        eventClick: function (info) {
          info.jsEvent.preventDefault();
        },
        // デフォルトの6週間表示を自動調整
        fixedWeekCount: false,
        headerToolbar: {
          start: "prev",
          center: "title",
          end: "next"
        },
        eventSources: [{
          googleCalendarApiKey: 'REDACTED_PUBLIC_SOURCE_GOOGLE_KEY',
          googleCalendarId: 'REDACTED_PUBLIC_HOLIDAY_CALENDAR_ID',
          className: 'event_holiday',
          color: "#ffaaaa",
          display: 'background',
        }],
        eventDidMount: function (info) {
          if (info.event._def.title == 'A') {
            info.el.style.background = '#fff';
          }
          if (info.event._def.title == 'B') {
            info.el.style.background = '#92d050';
          }
          if (info.event._def.title == 'C') {
            info.el.style.background = '#ffbf00';
          }
          if (info.event._def.title == 'D') {
            info.el.style.background = '#ff2828';
          }
          if (info.event._def.title == 'E') {
            info.el.style.background = '#fbf341';
          }
          //祝日の日付を赤に
          const items = document.querySelectorAll("div");
          for (let item of items) {
            if (item.classList.contains("event_holiday")) {
              item.closest('.fc-daygrid-day').classList.add('is_holiday');
            }
          }
        }
      });
//URLにen、cn、koが含まれる場合
if (location.href.indexOf('/en/') !== -1) {
 calendar3.setOption('locale', 'en');
} else if (location.href.indexOf('/cn/') !== -1) {
 calendar3.setOption('locale', 'en');
} else if (location.href.indexOf('/tw/') !== -1) {
 calendar3.setOption('locale', 'en');
} else if (location.href.indexOf('/ko/') !== -1) {
 calendar3.setOption('locale', 'en');
} else {
 calendar3.setOption('locale', 'jp');
}
      const calendar4 = new FullCalendar.Calendar(calendarEl4, {
        initialView: 'dayGridMonth',
        timeZone: "Asia/Tokyo",
        height: "auto",
        validRange: {
          start: '2026-10-01',
          end: '2027-04-01'
        },
        events: calendarEvents4,
        dayCellContent: function (arg) {
          return arg.date.getDate();
        },
        // デフォルトの6週間表示を自動調整
        fixedWeekCount: false,
        headerToolbar: {
          start: "prev",
          center: "title",
          end: "next"
        },
        eventSources: [{
          googleCalendarApiKey: 'REDACTED_PUBLIC_SOURCE_GOOGLE_KEY',
          googleCalendarId: 'REDACTED_PUBLIC_HOLIDAY_CALENDAR_ID',
          className: 'event_holiday',
          color: "#ffaaaa",
          display: 'background',
        }],
        eventDidMount: function (info) {
          if (info.event._def.title == 'A') {
            info.el.style.background = '#fff';
          }
          if (info.event._def.title == 'B') {
            info.el.style.background = '#92d050';
          }
          if (info.event._def.title == 'C') {
            info.el.style.background = '#fec000';
          }
          if (info.event._def.title == 'D') {
            info.el.style.background = '#ff2828';
          }
          if (info.event._def.title == 'E') {
            info.el.style.background = '#fbf341';
          }
          //祝日の日付を赤に
          const items = document.querySelectorAll("div");
          for (let item of items) {
            if (item.classList.contains("event_holiday")) {
              item.closest('.fc-daygrid-day').classList.add('is_holiday');
            }
          }
        }
       });
//URLにen、cn、koが含まれる場合
if (location.href.indexOf('/en/') !== -1) {
 calendar4.setOption('locale', 'en');
} else if (location.href.indexOf('/cn/') !== -1) {
 calendar4.setOption('locale', 'en');
} else if (location.href.indexOf('/tw/') !== -1) {
 calendar4.setOption('locale', 'en');
} else if (location.href.indexOf('/ko/') !== -1) {
 calendar4.setOption('locale', 'en');
} else {
 calendar4.setOption('locale', 'jp');
}
      calendar3.render();
      calendar4.render();
    });
