import { createRouter, createWebHistory } from 'vue-router';
import Base64Tools from '../components/Base64Tools.vue';
import URLTools from '../components/URLTools.vue';
import DateTimeTools from '../components/DateTimeTools.vue';
import JsonTools from '../components/JsonTools.vue';
import Sha1Tools from '../components/Sha1Tools.vue';
import MD5Tools from '../components/MD5Tools.vue';
import PortsTools from '../components/PortsTools.vue';
import ImageTools from '../components/ImageTools.vue';
import Home from '../views/Home.vue'
import { homeMeta, toolMetaMap } from '../constants/tool-meta'

const routes = [
    { path: '/', name: 'Home', component: Home, meta: homeMeta },
    { path: '/base64', name: 'Base64Tools', component: Base64Tools, meta: toolMetaMap['/base64'] },
    { path: '/url', name: 'URLTools', component: URLTools, meta: toolMetaMap['/url'] },
    { path: '/datetime', name: 'DateTimeTools', component: DateTimeTools, meta: toolMetaMap['/datetime'] },
    { path: '/json', name: 'JsonTools', component: JsonTools, meta: toolMetaMap['/json'] },
    { path: '/sha1', name: 'Sha1Tools', component: Sha1Tools, meta: toolMetaMap['/sha1'] },
    { path: '/md5', name: 'MD5Tools', component: MD5Tools, meta: toolMetaMap['/md5'] },
    { path: '/qrcode', name: 'QrCode', component: () => import('../views/QrCode.vue'), meta: toolMetaMap['/qrcode'] },
    { path: '/ports', name: 'PortsTools', component: PortsTools, meta: toolMetaMap['/ports'] },
    { path: '/image', name: 'ImageTools', component: ImageTools, meta: toolMetaMap['/image'] },

];

const router = createRouter({
    history: createWebHistory(),
    routes
});

export default router;
